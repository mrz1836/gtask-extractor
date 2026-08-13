package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		t.Run(tc.goos, func(t *testing.T) {
			name, args := browserCommand(tc.goos, url)
			assert.Equal(t, tc.wantName, name)
			assert.Equal(t, tc.wantArgs, args)
		})
	}
}

// TestOpenBrowserDefaultCanceledContext drives the error path without ever
// launching a real browser: exec.CommandContext checks the context before it
// forks, so an already-canceled context makes Start fail immediately.
func TestOpenBrowserDefaultCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.Error(t, openBrowserDefault(ctx, "https://example.com/auth"))
}
