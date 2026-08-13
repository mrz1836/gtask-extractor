package auth

import (
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
