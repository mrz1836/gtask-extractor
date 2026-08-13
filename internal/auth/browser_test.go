package auth

import (
	"context"
	"slices"
	"testing"
)

func TestBrowserCommand(t *testing.T) {
	const url = "https://example.com/auth?x=1"

	cases := []struct {
		goos     string
		wantName string
		wantArgs []string
	}{
		{"darwin", "open", []string{url}},
		{"windows", "rundll32", []string{"url.dll,FileProtocolHandler", url}},
		{"linux", "xdg-open", []string{url}},
		{"freebsd", "xdg-open", []string{url}}, // default branch
	}
	for _, tc := range cases {
		name, args := browserCommand(tc.goos, url)
		if name != tc.wantName {
			t.Errorf("%s: name = %q, want %q", tc.goos, name, tc.wantName)
		}

		if !slices.Equal(args, tc.wantArgs) {
			t.Errorf("%s: args = %v, want %v", tc.goos, args, tc.wantArgs)
		}
	}
}

// TestOpenBrowserDefaultCanceledContext drives the error path without ever
// launching a real browser: exec.CommandContext checks the context before it
// forks, so an already-canceled context makes Start fail immediately.
func TestOpenBrowserDefaultCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := openBrowserDefault(ctx, "https://example.com/auth"); err == nil {
		t.Error("openBrowserDefault with a canceled context should return an error")
	}
}
