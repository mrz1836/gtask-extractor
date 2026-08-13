package auth

import (
	"context"
	"os/exec"
	"runtime"
)

// openBrowserDefault opens url in the user's default browser. Callers treat
// failure as non-fatal (the URL is always printed to the terminal as well).
func openBrowserDefault(ctx context.Context, url string) error {
	name, args := browserCommand(runtime.GOOS, url)
	//nolint:gosec // G204: command name and args come from a fixed GOOS allow-list, not user input
	cmd := exec.CommandContext(ctx, name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	// The launcher (open/xdg-open/rundll32) forks the browser and exits almost
	// immediately; reap it so we don't leak a zombie or the CommandContext
	// watcher goroutine.
	go func() { _ = cmd.Wait() }()

	return nil
}

// browserCommand returns the command and arguments used to open a URL on the
// given GOOS. It is pure (no side effects) so it can be unit-tested directly.
func browserCommand(goos, url string) (name string, args []string) {
	switch goos {
	case "darwin":
		return "open", []string{url}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default: // linux, *bsd, etc.
		return "xdg-open", []string{url}
	}
}
