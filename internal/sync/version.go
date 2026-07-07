package sync

import (
	"os/exec"
	"regexp"
	"strconv"
	gosync "sync"
)

// macOS ships openrsync (Ventura+) or Apple's rsync 2.6.9, neither of which
// understands --info=progress2 (upstream rsync >= 3.1.0 only). Probe the
// user's rsync once and fall back to the universally supported --progress,
// which reports per-file percentages that parsePercent handles the same way.

var progressFlagOnce gosync.Once
var progressFlagVal string

// progressFlag returns the progress option supported by the rsync on PATH.
func progressFlag() string {
	progressFlagOnce.Do(func() {
		progressFlagVal = "--progress"
		out, err := exec.Command("rsync", "--version").Output()
		if err == nil && supportsInfoProgress2(out) {
			progressFlagVal = "--info=progress2"
		}
	})
	return progressFlagVal
}

var rsyncVersionRe = regexp.MustCompile(`rsync\s+version\s+v?(\d+)\.(\d+)`)

// supportsInfoProgress2 reports whether `rsync --version` output identifies an
// upstream rsync >= 3.1.0. openrsync advertises "rsync ... compatible" version
// strings, so it is rejected by name before the version check.
func supportsInfoProgress2(versionOut []byte) bool {
	s := string(versionOut)
	if regexp.MustCompile(`(?i)openrsync`).MatchString(s) {
		return false
	}
	m := rsyncVersionRe.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return major > 3 || (major == 3 && minor >= 1)
}
