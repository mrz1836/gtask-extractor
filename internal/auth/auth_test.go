package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

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

func TestConfigParsesDesktopCreds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")

	require.NoError(t, os.WriteFile(path, []byte(desktopCreds), 0o600))

	cfg, err := Config(path)
	require.NoError(t, err)
	assert.Equal(t, "test-client-id.apps.googleusercontent.com", cfg.ClientID)
	assert.Equal(t, []string{"https://www.googleapis.com/auth/tasks.readonly"}, cfg.Scopes)
	assert.Equal(t, "https://oauth2.googleapis.com/token", cfg.Endpoint.TokenURL)
}

func TestConfigMissingFile(t *testing.T) {
	_, err := Config(filepath.Join(t.TempDir(), "nope.json"))
	require.ErrorIs(t, err, ErrCredentialsNotFound)
}

func TestConfigInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")

	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o600))

	_, err := Config(path)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrCredentialsNotFound)
}

func TestTokenRoundTripAndPerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	want := &oauth2.Token{
		AccessToken:  "at-123",
		RefreshToken: "rt-456",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour).Round(time.Second),
	}
	require.NoError(t, saveToken(path, want))

	info, err := os.Stat(path)
	require.NoError(t, err)

	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	got, err := loadToken(path)
	require.NoError(t, err)
	assert.Equal(t, want.AccessToken, got.AccessToken)
	assert.Equal(t, want.RefreshToken, got.RefreshToken)
}

func TestLoadTokenMissingAndCorrupt(t *testing.T) {
	_, err := loadToken(filepath.Join(t.TempDir(), "absent.json"))
	require.ErrorIs(t, err, os.ErrNotExist)

	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	require.NoError(t, os.WriteFile(path, []byte("{bad"), 0o600))

	_, err = loadToken(path)
	require.Error(t, err)
	assert.NotErrorIs(t, err, os.ErrNotExist)
}

func TestClientUsesCachedToken(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token.json")

	require.NoError(t, saveToken(tokenPath, &oauth2.Token{
		AccessToken:  "cached",
		RefreshToken: "r",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}))

	cfg := &oauth2.Config{
		ClientID: "cid",
		Endpoint: oauth2.Endpoint{TokenURL: "https://example.invalid/token"},
	}
	// A cached, unexpired token means no browser/network is touched.
	c, err := Client(context.Background(), cfg, tokenPath, io.Discard)
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestClientCorruptTokenIsFriendly(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token.json")

	require.NoError(t, os.WriteFile(tokenPath, []byte("{bad"), 0o600))

	_, err := Client(context.Background(), &oauth2.Config{}, tokenPath, io.Discard)
	require.ErrorContains(t, err, "delete it")
}

func TestCallbackHandler(t *testing.T) {
	const state = "expected-state"

	tests := []struct {
		name      string
		path      string
		query     string
		wantCode  string
		wantErr   bool
		wantHTTP  int // 0 == default 200
		sendsNone bool
	}{
		{name: "happy", path: "/callback", query: "state=expected-state&code=auth-code", wantCode: "auth-code"},
		{name: "bad state", path: "/callback", query: "state=wrong&code=auth-code", wantErr: true},
		{name: "access denied", path: "/callback", query: "error=access_denied", wantErr: true},
		{name: "missing code", path: "/callback", query: "state=expected-state", wantErr: true},
		{name: "other path", path: "/favicon.ico", query: "", wantHTTP: http.StatusNotFound, sendsNone: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := make(chan callbackResult, 1)
			h := callbackHandler(state, ch)

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path+"?"+tc.query, nil)
			h(rr, req)

			if tc.wantHTTP != 0 {
				assert.Equal(t, tc.wantHTTP, rr.Code)
			}

			if tc.sendsNone {
				select {
				case res := <-ch:
					t.Errorf("expected no result, got %+v", res)
				default:
				}

				return
			}

			select {
			case res := <-ch:
				if tc.wantErr {
					require.Error(t, res.err)
				}

				if !tc.wantErr {
					assert.Equal(t, tc.wantCode, res.code)
				}
			case <-time.After(time.Second):
				t.Error("handler did not report a result")
			}
		})
	}
}

// TestAuthorizeFullFlow exercises the entire loopback + PKCE flow with a mocked
// browser and an httptest token endpoint. No real network access occurs.
func TestAuthorizeFullFlow(t *testing.T) {
	var gotForm url.Values

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(t, r.ParseForm())

		gotForm = r.Form

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "final-access-token",
			"refresh_token": "final-refresh-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer tokenSrv.Close()

	cfg := &oauth2.Config{
		ClientID:     "cid",
		ClientSecret: "secret",
		Scopes:       []string{"https://www.googleapis.com/auth/tasks.readonly"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.example.com/o/oauth2/auth",
			TokenURL: tokenSrv.URL,
		},
	}

	// Mock the browser: parse the auth URL, then "redirect" to the loopback
	// callback with a matching state and a fake code.
	open := func(_ context.Context, rawURL string) error {
		u, err := url.Parse(rawURL)
		if err != nil {
			return err
		}

		q := u.Query()
		redirect := q.Get("redirect_uri")
		state := q.Get("state")

		assert.NotEmpty(t, q.Get("code_challenge"))
		assert.Equal(t, "S256", q.Get("code_challenge_method"))

		cb := redirect + "?state=" + url.QueryEscape(state) + "&code=fake-code"

		resp, err := http.Get(cb)
		if err != nil {
			return err
		}

		return resp.Body.Close()
	}

	tok, err := authorize(context.Background(), cfg, io.Discard, open)
	require.NoError(t, err)
	assert.Equal(t, "final-access-token", tok.AccessToken)
	assert.Equal(t, "final-refresh-token", tok.RefreshToken)

	// PKCE verifier must have been posted to the token endpoint.
	assert.NotEmpty(t, gotForm.Get("code_verifier"))
	assert.Equal(t, "fake-code", gotForm.Get("code"))
	assert.Equal(t, "authorization_code", gotForm.Get("grant_type"))
}

// stubSource returns a preset sequence of tokens, one per Token() call.
type stubSource struct {
	toks []*oauth2.Token
	i    int
}

func (s *stubSource) Token() (*oauth2.Token, error) {
	tok := s.toks[s.i]
	if s.i < len(s.toks)-1 {
		s.i++
	}

	return tok, nil
}

func TestPersistingTokenSourceRewritesOnRefresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")

	first := &oauth2.Token{AccessToken: "first", RefreshToken: "r", TokenType: "Bearer"}
	second := &oauth2.Token{AccessToken: "second", RefreshToken: "r", TokenType: "Bearer"}

	p := &persistingTokenSource{
		base: &stubSource{toks: []*oauth2.Token{first, second}},
		path: path,
		warn: io.Discard,
		last: first.AccessToken, // pretend "first" is already on disk
	}

	// First call returns the already-known token: no write expected.
	_, err := p.Token()
	require.NoError(t, err)

	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)

	// Second call returns a refreshed token: it must be persisted.
	_, err = p.Token()
	require.NoError(t, err)

	saved, err := loadToken(path)
	require.NoError(t, err)
	assert.Equal(t, "second", saved.AccessToken)
}

// TestPersistingTokenSourceBestEffort verifies that a token-cache write failure
// is non-fatal: the freshly refreshed token is still returned, and a warning is
// emitted rather than an error.
func TestPersistingTokenSourceBestEffort(t *testing.T) {
	// A path inside a directory that does not exist makes saveToken fail.
	badPath := filepath.Join(t.TempDir(), "missing-subdir", "token.json")

	var warn bytes.Buffer

	p := &persistingTokenSource{
		base: &stubSource{toks: []*oauth2.Token{{AccessToken: "fresh", TokenType: "Bearer"}}},
		path: badPath,
		warn: &warn,
		last: "", // force a persist attempt
	}

	tok, err := p.Token()
	require.NoError(t, err)
	assert.Equal(t, "fresh", tok.AccessToken)
	assert.Contains(t, warn.String(), "could not update cached token")
}
