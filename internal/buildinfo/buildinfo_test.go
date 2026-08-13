package buildinfo

import "testing"

func TestUserAgent(t *testing.T) {
	old := Version
	defer func() { Version = old }()

	Version = "1.2.3"

	if got, want := UserAgent(), "gtasks/1.2.3"; got != want {
		t.Errorf("UserAgent() = %q, want %q", got, want)
	}
}
