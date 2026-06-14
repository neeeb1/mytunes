// Package remote scans the server's music library over the user's ssh binary.
package remote

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/neeeb1/mytunes/internal/diff"
)

// Scanner runs a single per-file `find` on the server and parses its
// NUL-delimited output. SSH auth (keys, agent, known_hosts) is left entirely to
// the ssh binary.
type Scanner struct {
	Remote string // user@host
	Path   string // music root, e.g. /srv/music
}

// Scan returns one FileEntry per track file. It issues exactly one SSH call:
//
//	find <path> -mindepth 3 -type f -printf '%s\t%p\0'
//
// -mindepth 3 matches the verified <Artist>/<Album>/<File> layout.
func (s Scanner) Scan(ctx context.Context) ([]diff.FileEntry, error) {
	findExpr := fmt.Sprintf(`find %s -mindepth 3 -type f -printf '%%s\t%%p\0'`, shellQuote(s.Path))
	// All args are passed via argv; the remote shell only sees the find
	// expression we built, with the path single-quoted.
	cmd := exec.CommandContext(ctx, "ssh", s.Remote, findExpr)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ssh scan failed: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return parse(out, s.Path)
}

// parse splits NUL-delimited records as bytes (never on whitespace) so that
// names with spaces, $, quotes, smart-quotes and unicode round-trip cleanly.
func parse(out []byte, root string) ([]diff.FileEntry, error) {
	prefix := strings.TrimRight(root, "/") + "/"
	var entries []diff.FileEntry

	for _, rec := range bytes.Split(out, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		tab := bytes.IndexByte(rec, '\t')
		if tab < 0 {
			return nil, fmt.Errorf("malformed record: %q", rec)
		}
		size, err := strconv.ParseInt(string(rec[:tab]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad size in %q: %w", rec, err)
		}
		full := string(rec[tab+1:])

		rel := strings.TrimPrefix(full, prefix)
		artist, album, file, ok := splitArtistAlbumFile(rel)
		if !ok {
			continue // not the expected Artist/Album/File depth; skip
		}
		entries = append(entries, diff.FileEntry{
			Artist: artist, Album: album, File: file, Size: size,
		})
	}
	return entries, nil
}

// splitArtistAlbumFile splits "Artist/Album/File" keeping anything deeper as
// part of the file name (so multi-disc subdir layouts don't break parsing).
func splitArtistAlbumFile(rel string) (artist, album, file string, ok bool) {
	first := strings.IndexByte(rel, '/')
	if first < 0 {
		return "", "", "", false
	}
	rest := rel[first+1:]
	second := strings.IndexByte(rest, '/')
	if second < 0 {
		return "", "", "", false
	}
	return rel[:first], rest[:second], rest[second+1:], true
}

// shellQuote single-quotes a string for safe interpolation into the remote find
// expression (the only place we build a remote command string).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
