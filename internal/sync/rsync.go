// Package sync turns the user's queued diff into concrete filesystem work: a
// delete-first pass (local rm) followed by an rsync copy/update pass. It shells
// out to the user's rsync; it never reimplements transfer logic.
package sync

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/neeeb1/mytunes/internal/diff"
)

// baseArgs keep times but skip perms/owner/group that FAT/exFAT can't store.
var baseArgs = []string{"-rt", "--modify-window=1", "--no-perms", "--no-owner", "--no-group"}

// Job is the fully-resolved work derived from the diff tree.
type Job struct {
	Remote     string   // user@host
	RemotePath string   // e.g. /srv/music
	Dest       string   // destination root
	ExtraArgs  []string // rsync_extra_args from config
	DryRun     bool

	// CopyPaths are "Artist/Album" relative paths fed to rsync --files-from.
	CopyPaths []string
	// DeleteDirs are absolute destination dirs to remove (guarded to Dest).
	DeleteDirs []string
}

// BuildJob walks the tree and collects copy/update scopes and delete targets.
func BuildJob(t *diff.Tree, remote, remotePath, dest string, extra []string, dryRun bool) Job {
	j := Job{
		Remote:     remote,
		RemotePath: remotePath,
		Dest:       filepath.Clean(dest),
		ExtraArgs:  extra,
		DryRun:     dryRun,
	}
	for _, ar := range t.Artists {
		for _, al := range ar.Albums {
			switch al.Action() {
			case diff.Copy, diff.Update:
				j.CopyPaths = append(j.CopyPaths, al.Artist+"/"+al.Name)
			case diff.Delete:
				j.DeleteDirs = append(j.DeleteDirs, filepath.Join(j.Dest, al.Artist, al.Name))
			}
		}
	}
	return j
}

// withinDest reports whether target resolves to a strict subdirectory of dest.
// This is the guard that keeps deletes from ever escaping the destination.
func withinDest(dest, target string) bool {
	dest = filepath.Clean(dest)
	target = filepath.Clean(target)
	if target == dest {
		return false // never delete the destination root itself
	}
	rel, err := filepath.Rel(dest, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// CheckDeletes verifies every delete target is contained in Dest, returning the
// first offender. Call before RunDeletes.
func (j Job) CheckDeletes() error {
	for _, d := range j.DeleteDirs {
		if !withinDest(j.Dest, d) {
			return fmt.Errorf("refusing to delete %q: outside destination %q", d, j.Dest)
		}
	}
	return nil
}

// RunDeletes removes each queued album dir locally. progress is called after
// each removal with (done, total). The server is never touched.
func (j Job) RunDeletes(progress func(done, total int)) error {
	if err := j.CheckDeletes(); err != nil {
		return err
	}
	total := len(j.DeleteDirs)
	for i, d := range j.DeleteDirs {
		if !j.DryRun {
			if err := os.RemoveAll(d); err != nil {
				return fmt.Errorf("delete %q: %w", d, err)
			}
		}
		if progress != nil {
			progress(i+1, total)
		}
	}
	return nil
}

// writeFilesFrom writes CopyPaths to a temp file for rsync --files-from and
// returns its path plus a cleanup func.
func (j Job) writeFilesFrom() (string, func(), error) {
	f, err := os.CreateTemp("", "mytunes-queued-*.txt")
	if err != nil {
		return "", func() {}, err
	}
	w := bufio.NewWriter(f)
	for _, p := range j.CopyPaths {
		fmt.Fprintln(w, p)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", func() {}, err
	}
	f.Close()
	cleanup := func() { os.Remove(f.Name()) }
	return f.Name(), cleanup, nil
}

// rsyncArgs assembles the argv for a copy/update run. extraFront lets callers
// inject flags like --dry-run/--stats or --info=progress2.
func (j Job) rsyncArgs(filesFrom string, extraFront ...string) []string {
	args := append([]string{}, baseArgs...)
	args = append(args, j.ExtraArgs...)
	args = append(args, extraFront...)
	args = append(args,
		"--files-from="+filesFrom,
		j.Remote+":"+strings.TrimRight(j.RemotePath, "/")+"/",
		j.Dest+"/",
	)
	return args
}

// DryRunBytes runs `rsync --dry-run --stats` over the copy/update scope and
// returns the precise "Total transferred file size" for the Confirm summary.
func (j Job) DryRunBytes(ctx context.Context) (int64, error) {
	if len(j.CopyPaths) == 0 {
		return 0, nil
	}
	ff, cleanup, err := j.writeFilesFrom()
	if err != nil {
		return 0, err
	}
	defer cleanup()

	cmd := exec.CommandContext(ctx, "rsync", j.rsyncArgs(ff, "--dry-run", "--stats")...)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("rsync dry-run failed: %w", err)
	}
	return parseTransferredBytes(out)
}

// RunCopy executes the live copy/update pass, streaming rsync output and
// reporting overall percent (0..100) via progress and the path of the file
// currently being transferred via onFile. Either callback may be nil.
//
// -v makes rsync print each transferred path on its own line, interleaved with
// the --info=progress2 aggregate line, so we can show what's moving even while
// the overall percentage (across the whole job) climbs slowly.
func (j Job) RunCopy(ctx context.Context, progress func(percent float64), onFile func(name string)) error {
	if len(j.CopyPaths) == 0 {
		return nil
	}
	ff, cleanup, err := j.writeFilesFrom()
	if err != nil {
		return err
	}
	defer cleanup()

	front := []string{"-v", "--info=progress2"}
	if j.DryRun {
		front = append(front, "--dry-run")
	}
	cmd := exec.CommandContext(ctx, "rsync", j.rsyncArgs(ff, front...)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	sc := bufio.NewScanner(stdout)
	sc.Split(scanProgress)
	for sc.Scan() {
		line := sc.Text()
		if p, ok := parsePercent(line); ok {
			if progress != nil {
				progress(p)
			}
			continue
		}
		if name, ok := transferredFile(line); ok && onFile != nil {
			onFile(name)
		}
	}
	return cmd.Wait()
}

// transferredFile reports whether a -v output line names a file rsync is
// transferring (as opposed to a progress line, a blank line, or a status
// message). Real paths in the <Artist>/<Album> layout always contain a slash; the status
// lines rsync -v emits start with a known keyword (and one, "...bytes/sec",
// would otherwise sneak past the slash check).
func transferredFile(line string) (string, bool) {
	line = strings.TrimRight(line, "\r")
	if line == "" || !strings.Contains(line, "/") {
		return "", false
	}
	for _, p := range []string{"sending ", "sent ", "total ", "created ", "deleting "} {
		if strings.HasPrefix(line, p) {
			return "", false
		}
	}
	return line, true
}

// parseTransferredBytes extracts "Total transferred file size: N bytes" from
// rsync --stats output (matching the original script's awk).
func parseTransferredBytes(out []byte) (int64, error) {
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "Total transferred file size") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		val := line[colon+1:]
		val = strings.ReplaceAll(val, ",", "")
		val = strings.ReplaceAll(val, "bytes", "")
		val = strings.TrimSpace(val)
		return strconv.ParseInt(val, 10, 64)
	}
	return 0, nil
}

// parsePercent pulls the NN% token out of an --info=progress2 line.
func parsePercent(line string) (float64, bool) {
	for _, tok := range strings.Fields(line) {
		if strings.HasSuffix(tok, "%") {
			n, err := strconv.ParseFloat(strings.TrimSuffix(tok, "%"), 64)
			if err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

// scanProgress is a bufio.SplitFunc that breaks on both \n and \r so rsync's
// in-place progress updates (carriage-return refreshed) are seen individually.
func scanProgress(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}
