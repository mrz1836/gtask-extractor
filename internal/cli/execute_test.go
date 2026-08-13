package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserAgent(t *testing.T) {
	assert.Equal(t, "gtask-extractor/1.2.3", userAgent("1.2.3"))
}

func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil is ok", nil, exitOK},
		{"canceled is aborted", context.Canceled, exitAborted},
		{"deadline is aborted", context.DeadlineExceeded, exitAborted},
		{"coded keeps its code", coded(exitAuth, errors.New("x")), exitAuth},
		{"wrapped coded is unwrapped", fmt.Errorf("wrap: %w", coded(exitOutput, errors.New("x"))), exitOutput},
		{"plain error is a usage error", errors.New("plain"), exitUsage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ExitCode(tc.err))
		})
	}
}

// silenceStdio points os.Stdout and os.Stderr at /dev/null and restores them
// (and os.Args) when the test ends. Execute reads os.Args and writes to the real
// process streams, so tests that drive it must isolate both. Redirecting stdout
// also guarantees IsInteractive reports false regardless of where the test runs.
func silenceStdio(t *testing.T) {
	t.Helper()

	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	require.NoError(t, err)

	origOut, origErr, origArgs := os.Stdout, os.Stderr, os.Args
	os.Stdout, os.Stderr = devnull, devnull

	t.Cleanup(func() {
		os.Stdout, os.Stderr, os.Args = origOut, origErr, origArgs
		_ = devnull.Close()
	})
}

func TestExecuteVersion(t *testing.T) {
	// Keep the passive self-update check from touching the network.
	t.Setenv("GTASK_EXTRACTOR_NO_UPDATE_CHECK", "1")
	t.Setenv("NO_UPDATE_CHECK", "1")
	silenceStdio(t)

	os.Args = []string{"gtask-extractor", "version"}

	require.NoError(t, Execute(BuildInfo{Version: "1.2.3"}))
}

func TestExecuteNotInteractiveErrors(t *testing.T) {
	t.Setenv("GTASK_EXTRACTOR_NO_UPDATE_CHECK", "1")
	t.Setenv("NO_UPDATE_CHECK", "1")
	silenceStdio(t)

	// With no subcommand the root runs the interactive flow, which fails fast
	// because the test process is not attached to a TTY. Execute prints the
	// error and returns it for main to map to an exit code.
	os.Args = []string{"gtask-extractor"}

	err := Execute(BuildInfo{})
	require.Error(t, err)
	require.ErrorIs(t, err, errNotInteractive)
	assert.Equal(t, exitUsage, ExitCode(err))
}
