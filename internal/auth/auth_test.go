package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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
	if err := os.WriteFile(path, []byte(desktopCreds), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Config(path)
	if err != nil {
		t.Fatalf("Config: %v", err)
	}

	if cfg.ClientID != "test-client-id.apps.googleusercontent.com" {
		t.Errorf("ClientID = %q", cfg.ClientID)
	}

	if len(cfg.Scopes) != 1 || cfg.Scopes[0] != "https://www.googleapis.com/auth/tasks.readonly" {
		t.Errorf("Scopes = %v, want the read-only Tasks scope only", cfg.Scopes)
	}

	if cfg.Endpoint.TokenURL != "https://oauth2.googleapis.com/token" {
		t.Errorf("TokenURL = %q", cfg.Endpoint.TokenURL)
	}
}

func TestConfigMissingFile(t *testing.T) {
	_, err := Config(filepath.Join(t.TempDir(), "nope.json"))
	if !errors.Is(err, ErrCredentialsNotFound) {
		t.Fatalf("err = %v, want ErrCredentialsNotFound", err)
	}
}

func TestConfigInvalidJSON(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "credentials.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Config(path); err == nil || errors.Is(err, ErrCredentialsNotFound) {
		t.Fatalf("err = %v, want a parse error", err)
	}
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
	if err := saveToken(path, want); err != nil {
		t.Fatalf("saveToken: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("token perm = %o, want 600", perm)
		}
	}

	got, err := loadToken(path)
	if err != nil {
		t.Fatalf("loadToken: %v", err)
	}

	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken {
		t.Errorf("round-trip mismatch: got %+v", got)
	}
}

func TestLoadTokenMissingAndCorrupt(t *testing.T) {
	if _, err := loadToken(filepath.Join(t.TempDir(), "absent.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing token err = %v, want os.ErrNotExist", err)
	}

	dir := t.TempDir()

	path := filepath.Join(dir, "token.json")
	if err := os.WriteFile(path, []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := loadToken(path); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Errorf("corrupt token err = %v, want a decode error", err)
	}
}

func TestClientUsesCachedToken(t *testing.T) {
	dir := t.TempDir()

	tokenPath := filepath.Join(dir, "token.json")
	if err := saveToken(tokenPath, &oauth2.Token{
		AccessToken:  "cached",
		RefreshToken: "r",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	cfg := &oauth2.Config{
		ClientID: "cid",
		Endpoint: oauth2.Endpoint{TokenURL: "https://example.invalid/token"},
	}
	// A cached, unexpired token means no browser/network is touched.
	c, err := Client(context.Background(), cfg, tokenPath, io.Discard)
	if err != nil {
		t.Fatalf("Client: %v", err)
	}

	if c == nil {
		t.Fatal("Client returned a nil *http.Client")
	}
}

func TestClientCorruptTokenIsFriendly(t *testing.T) {
	dir := t.TempDir()

	tokenPath := filepath.Join(dir, "token.json")
	if err := os.WriteFile(tokenPath, []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Client(context.Background(), &oauth2.Config{}, tokenPath, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "delete it") {
		t.Fatalf("err = %v, want a friendly 'delete it and re-run' message", err)
	}
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

			if tc.wantHTTP != 0 && rr.Code != tc.wantHTTP {
				t.Errorf("HTTP status = %d, want %d", rr.Code, tc.wantHTTP)
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
				if tc.wantErr && res.err == nil {
					t.Errorf("expected error result, got code %q", res.code)
				}

				if !tc.wantErr && res.code != tc.wantCode {
					t.Errorf("code = %q, want %q", res.code, tc.wantCode)
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
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}

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
		if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
			t.Errorf("auth URL missing PKCE challenge: %s", rawURL)
		}

		cb := redirect + "?state=" + url.QueryEscape(state) + "&code=fake-code"

		resp, err := http.Get(cb)
		if err != nil {
			return err
		}

		return resp.Body.Close()
	}

	tok, err := authorize(context.Background(), cfg, io.Discard, open)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}

	if tok.AccessToken != "final-access-token" || tok.RefreshToken != "final-refresh-token" {
		t.Errorf("token = %+v, want the canned token", tok)
	}

	// PKCE verifier must have been posted to the token endpoint.
	if gotForm.Get("code_verifier") == "" {
		t.Error("token exchange did not include code_verifier (PKCE)")
	}

	if gotForm.Get("code") != "fake-code" {
		t.Errorf("exchange code = %q, want fake-code", gotForm.Get("code"))
	}

	if gotForm.Get("grant_type") != "authorization_code" {
		t.Errorf("grant_type = %q", gotForm.Get("grant_type"))
	}
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
	if _, err := p.Token(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("token file should not exist yet (no refresh happened)")
	}

	// Second call returns a refreshed token: it must be persisted.
	if _, err := p.Token(); err != nil {
		t.Fatal(err)
	}

	saved, err := loadToken(path)
	if err != nil {
		t.Fatalf("expected persisted token: %v", err)
	}

	if saved.AccessToken != "second" {
		t.Errorf("persisted access token = %q, want second", saved.AccessToken)
	}
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
	if err != nil {
		t.Fatalf("Token() should not fail when persistence fails: %v", err)
	}

	if tok.AccessToken != "fresh" {
		t.Errorf("Token() = %q, want the valid refreshed token", tok.AccessToken)
	}

	if !strings.Contains(warn.String(), "could not update cached token") {
		t.Errorf("expected a non-fatal warning, got %q", warn.String())
	}
}
