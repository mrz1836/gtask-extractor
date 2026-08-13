package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, os.WriteFile(path, []byte(desktopCreds), 0o600))

	return path
}

// writeToken marshals tok to path in the same JSON shape auth.loadToken reads.
func writeToken(t *testing.T, path string, tok *oauth2.Token) {
	t.Helper()

	data, err := json.MarshalIndent(tok, "", "  ")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(path, data, 0o600))
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
	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestAuthenticateMissingCreds(t *testing.T) {
	dir := t.TempDir()
	opts := &options{
		credsPath: filepath.Join(dir, "nope.json"),
		tokenPath: filepath.Join(dir, "token.json"),
	}

	_, err := authenticate(context.Background(), opts, io.Discard)
	require.ErrorIs(t, err, auth.ErrCredentialsNotFound)

	assert.Equal(t, exitAuth, ExitCode(err))
	// The friendly message names the missing path so users know where to look.
	assert.Contains(t, err.Error(), "nope.json")
}

func TestAuthenticateInvalidCreds(t *testing.T) {
	dir := t.TempDir()

	credsPath := filepath.Join(dir, "credentials.json")
	require.NoError(t, os.WriteFile(credsPath, []byte("not json"), 0o600))

	opts := &options{credsPath: credsPath, tokenPath: filepath.Join(dir, "token.json")}

	_, err := authenticate(context.Background(), opts, io.Discard)
	require.Error(t, err)
	require.NotErrorIs(t, err, auth.ErrCredentialsNotFound)

	assert.Equal(t, exitAuth, ExitCode(err))
}

func TestAuthenticateCorruptToken(t *testing.T) {
	dir := t.TempDir()
	credsPath := writeCreds(t, dir)

	tokenPath := filepath.Join(dir, "token.json")
	require.NoError(t, os.WriteFile(tokenPath, []byte("{bad"), 0o600))

	opts := &options{credsPath: credsPath, tokenPath: tokenPath}

	_, err := authenticate(context.Background(), opts, io.Discard)
	require.Equal(t, exitAuth, ExitCode(err))

	// The hint tells the user how to recover from a bad token cache.
	assert.Contains(t, err.Error(), "re-run")
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
	require.ErrorIs(t, err, auth.ErrCredentialsNotFound)

	assert.Equal(t, exitAuth, ExitCode(err))
}
