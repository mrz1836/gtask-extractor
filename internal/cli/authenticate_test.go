package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	"github.com/mrz1836/gtask-extractor/internal/auth"
)

// desktopCreds is a minimal Google "Desktop app" credentials.json, enough for
// auth.Config to parse and build an oauth2.Config.
const desktopCreds = `{
  "installed": {
    "client_id": "test-client-id.apps.googleusercontent.com",
    "project_id": "gtasks-test",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "client_secret": "test-secret",
    "redirect_uris": ["http://localhost"]
  }
}`

// writeCreds writes desktopCreds to <dir>/credentials.json and returns the path.
func writeCreds(t *testing.T, dir string) string {
	t.Helper()

	path := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(path, []byte(desktopCreds), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

// writeToken marshals tok to path in the same JSON shape auth.loadToken reads.
func writeToken(t *testing.T, path string, tok *oauth2.Token) {
	t.Helper()

	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAuthenticateSuccess(t *testing.T) {
	dir := t.TempDir()
	credsPath := writeCreds(t, dir)
	tokenPath := filepath.Join(dir, "token.json")
	// A cached, unexpired token means no browser and no network are touched.
	writeToken(t, tokenPath, &oauth2.Token{
		AccessToken:  "cached",
		RefreshToken: "r",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	})

	opts := &options{credsPath: credsPath, tokenPath: tokenPath, version: "test"}

	client, err := authenticate(context.Background(), opts, io.Discard)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	if client == nil {
		t.Fatal("authenticate returned a nil client")
	}
}

func TestAuthenticateMissingCreds(t *testing.T) {
	dir := t.TempDir()
	opts := &options{
		credsPath: filepath.Join(dir, "nope.json"),
		tokenPath: filepath.Join(dir, "token.json"),
	}

	_, err := authenticate(context.Background(), opts, io.Discard)
	if !errors.Is(err, auth.ErrCredentialsNotFound) {
		t.Fatalf("err = %v, want auth.ErrCredentialsNotFound", err)
	}

	if got := ExitCode(err); got != exitAuth {
		t.Errorf("ExitCode = %d, want exitAuth (%d)", got, exitAuth)
	}
	// The friendly message names the missing path so users know where to look.
	if !strings.Contains(err.Error(), "nope.json") {
		t.Errorf("err = %v, want it to mention the missing credentials path", err)
	}
}

func TestAuthenticateInvalidCreds(t *testing.T) {
	dir := t.TempDir()

	credsPath := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(credsPath, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	opts := &options{credsPath: credsPath, tokenPath: filepath.Join(dir, "token.json")}

	_, err := authenticate(context.Background(), opts, io.Discard)
	if err == nil || errors.Is(err, auth.ErrCredentialsNotFound) {
		t.Fatalf("err = %v, want a parse error", err)
	}

	if got := ExitCode(err); got != exitAuth {
		t.Errorf("ExitCode = %d, want exitAuth (%d)", got, exitAuth)
	}
}

func TestAuthenticateCorruptToken(t *testing.T) {
	dir := t.TempDir()
	credsPath := writeCreds(t, dir)

	tokenPath := filepath.Join(dir, "token.json")
	if err := os.WriteFile(tokenPath, []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}

	opts := &options{credsPath: credsPath, tokenPath: tokenPath}

	_, err := authenticate(context.Background(), opts, io.Discard)
	if got := ExitCode(err); got != exitAuth {
		t.Fatalf("ExitCode = %d, want exitAuth (%d)", got, exitAuth)
	}
	// The hint tells the user how to recover from a bad token cache.
	if !strings.Contains(err.Error(), "re-run") {
		t.Errorf("err = %v, want a friendly re-authorize hint", err)
	}
}

func TestRunExportAuthFailure(t *testing.T) {
	// Valid usage (--all) but missing credentials: runExport must surface the
	// coded auth error before ever reaching the network.
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	dir := t.TempDir()
	opts := &options{
		credsPath: filepath.Join(dir, "nope.json"),
		tokenPath: filepath.Join(dir, "token.json"),
	}

	err := runExport(cmd, opts, nil, true)
	if !errors.Is(err, auth.ErrCredentialsNotFound) {
		t.Fatalf("err = %v, want auth.ErrCredentialsNotFound", err)
	}

	if got := ExitCode(err); got != exitAuth {
		t.Errorf("ExitCode = %d, want exitAuth (%d)", got, exitAuth)
	}
}
