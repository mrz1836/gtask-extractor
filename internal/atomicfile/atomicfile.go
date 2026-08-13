// Package atomicfile writes files atomically: it writes to a temporary file in
// the destination directory and renames it into place, so a reader never
// observes a partially written file. The rename is atomic when the temp file and
// destination share a filesystem, which they do because the temp file is created
// in the destination's own directory.
package atomicfile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Write writes data to path atomically with the given file permissions. It
// creates a temporary file (".gtasks-*.tmp") in path's directory, writes and
// chmods it, closes it, then renames it over path. The temp file is removed if
// anything fails before the rename. The destination directory must already
// exist; callers that need it created should MkdirAll first.
func Write(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".gtasks-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting output permissions: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("finalizing output %q: %w", path, err)
	}

	return nil
}
