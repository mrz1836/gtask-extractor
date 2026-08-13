package atomicfile

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrite(t *testing.T) {
	cases := []struct {
		name string
		perm fs.FileMode
	}{
		{name: "0644", perm: 0o644},
		{name: "0600", perm: 0o600},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "out.txt")
			data := []byte("hello atomic world\n")

			require.NoError(t, Write(path, data, tc.perm))

			// The renamed file is fully visible with the exact bytes written.
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, data, got)

			if runtime.GOOS != "windows" {
				info, statErr := os.Stat(path)
				require.NoError(t, statErr)
				assert.Equal(t, tc.perm, info.Mode().Perm())
			}

			// A successful write leaves no temp files behind.
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)

			for _, e := range entries {
				assert.Falsef(t, strings.HasPrefix(e.Name(), ".gtasks-"), "leftover temp file: %s", e.Name())
			}
		})
	}
}

func TestWriteOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	require.NoError(t, os.WriteFile(path, []byte("old contents"), 0o644))
	require.NoError(t, Write(path, []byte("new contents"), 0o644))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new contents", string(got))
}

func TestWriteMissingDirErrors(t *testing.T) {
	// CreateTemp fails when the destination directory does not exist, and no
	// temp file is left behind.
	path := filepath.Join(t.TempDir(), "missing-subdir", "out.txt")

	err := Write(path, []byte("x"), 0o600)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating temp file")
}
