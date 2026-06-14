// Package local scans the destination directory and reports its free space.
package local

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/neeeb1/mytunes/internal/diff"
)

// Scan walks dest and returns one FileEntry per file under the
// <Artist>/<Album>/<File> layout that mirrors the server. A missing dest is not
// an error — it yields zero entries (first sync to a fresh path).
func Scan(dest string) ([]diff.FileEntry, error) {
	dest = filepath.Clean(dest)
	info, err := os.Stat(dest)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	var entries []diff.FileEntry
	err = filepath.WalkDir(dest, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(dest, path)
		if rerr != nil {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 3 {
			return nil // shallower than Artist/Album/File; ignore
		}
		fi, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		entries = append(entries, diff.FileEntry{
			Artist: parts[0],
			Album:  parts[1],
			File:   strings.Join(parts[2:], "/"),
			Size:   fi.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}
