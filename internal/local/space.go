package local

import (
	"os"
	"path/filepath"
	"syscall"
)

// FreeBytes returns the bytes available to an unprivileged user on the
// filesystem backing dir. If dir does not yet exist, the nearest existing
// ancestor is measured (the dir will live on that filesystem once created).
func FreeBytes(dir string) (int64, error) {
	dir = filepath.Clean(dir)
	for {
		var st syscall.Statfs_t
		if err := syscall.Statfs(dir, &st); err == nil {
			return int64(st.Bavail) * int64(st.Bsize), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// reached root without success; surface the error
			var st syscall.Statfs_t
			err := syscall.Statfs(dir, &st)
			return 0, err
		}
		dir = parent
	}
}

// Writable reports whether dir exists and is a writable directory.
func Writable(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	// Probe by creating a temp file; cheaper than parsing mode bits across
	// ownership/ACL cases.
	f, err := os.CreateTemp(dir, ".mytunes-wtest-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}
