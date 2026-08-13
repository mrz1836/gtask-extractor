// Package auth implements the OAuth 2.0 flow gtasks uses to obtain read-only
// access to a personal Google account's Tasks data.
//
// The flow is an installed-app "loopback redirect" with PKCE: gtasks starts a
// throwaway HTTP server on 127.0.0.1, sends the user to Google's consent page,
// and receives the authorization code on the loopback address. The out-of-band
// (OOB) "paste this code" flow is deprecated by Google and intentionally not
// used here.
//
// The minted token is cached on disk (0600) and refreshed automatically; each
// refresh is written back so the on-disk token stays current across runs.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	tasks "google.golang.org/api/tasks/v1"

	"github.com/mrz1836/gtask-extractor/internal/atomicfile"
)

// ErrCredentialsNotFound is returned by Config when the credentials file is
// absent, so callers can print a friendly setup hint instead of a raw path
// error.
var ErrCredentialsNotFound = errors.New("credentials file not found")

// Sentinel errors for the OAuth callback outcomes.
var (
	errAuthDenied    = errors.New("authorization denied by Google")
	errStateMismatch = errors.New("state mismatch in OAuth callback")
	errNoAuthCode    = errors.New("no authorization code in OAuth callback")
)

// readHeaderTimeout bounds how long the loopback server waits for request
// headers, mitigating slow-header (Slowloris) stalls.
const readHeaderTimeout = 10 * time.Second

// Config loads a Google "Desktop app" credentials.json and requests only the
// read-only Tasks scope.
func Config(credsPath string) (*oauth2.Config, error) {
	//nolint:gosec // G304: credsPath is a user-supplied path to their own credentials file
	data, err := os.ReadFile(credsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrCredentialsNotFound
		}

		return nil, fmt.Errorf("reading credentials %q: %w", credsPath, err)
	}

	cfg, err := google.ConfigFromJSON(data, tasks.TasksReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("parsing credentials %q: %w", credsPath, err)
	}

	return cfg, nil
}

// Client returns an *http.Client authorized for the Tasks API.
//
// If a cached token exists it is used (and transparently refreshed); otherwise
// the interactive consent flow runs once and the resulting token is saved. The
// returned client re-persists the token whenever it is refreshed.
//
// promptWriter receives human-facing progress lines (typically os.Stderr).
func Client(ctx context.Context, cfg *oauth2.Config, tokenPath string, promptWriter io.Writer) (*http.Client, error) {
	if promptWriter == nil {
		promptWriter = io.Discard
	}

	tok, err := loadToken(tokenPath)
	switch {
	case err == nil:
		// Use the cached token.
	case errors.Is(err, os.ErrNotExist):
		// First run: obtain consent interactively.
		fresh, authErr := authorize(ctx, cfg, promptWriter, openBrowserDefault)
		if authErr != nil {
			return nil, authErr
		}

		if saveErr := saveToken(tokenPath, fresh); saveErr != nil {
			return nil, saveErr
		}

		tok = fresh
	default:
		// Present but unreadable/corrupt.
		return nil, fmt.Errorf("cached token %q is unreadable: %w; delete it and re-run to re-authorize", tokenPath, err)
	}

	src := &persistingTokenSource{
		base: cfg.TokenSource(ctx, tok),
		path: tokenPath,
		warn: promptWriter,
		last: tok.AccessToken,
	}

	return oauth2.NewClient(ctx, src), nil
}

// persistingTokenSource wraps a refreshing TokenSource and writes the token
// back to disk whenever a new access token is minted.
type persistingTokenSource struct {
	base oauth2.TokenSource
	path string
	warn io.Writer // non-nil; receives non-fatal persistence warnings

	mu   sync.Mutex
	last string
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if tok.AccessToken != p.last {
		// Persistence is best-effort: the refresh already succeeded and the
		// token is valid in memory, so a cache-write failure must not abort the
		// in-flight request. We warn and carry on; the next run re-refreshes.
		if saveErr := saveToken(p.path, tok); saveErr != nil {
			fmt.Fprintf(p.warn, "warning: could not update cached token %q: %v\n", p.path, saveErr)
		} else {
			p.last = tok.AccessToken
		}
	}

	return tok, nil
}

// callbackResult carries the outcome of the loopback OAuth redirect.
type callbackResult struct {
	code string
	err  error
}

// authorize runs the loopback + PKCE consent flow and returns a fresh token.
// open is injected so tests can drive the callback without a real browser.
func authorize(ctx context.Context, cfg *oauth2.Config, w io.Writer, open func(context.Context, string) error) (*oauth2.Token, error) {
	var lc net.ListenConfig

	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("starting loopback listener: %w", err)
	}

	defer func() { _ = ln.Close() }()

	port := ln.Addr().(*net.TCPAddr).Port

	// Work on a copy so the caller's config is not mutated.
	local := *cfg
	local.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// A cryptographically random, URL-safe state value (crypto/rand.Text).
	state := rand.Text()
	verifier := oauth2.GenerateVerifier()

	authURL := local.AuthCodeURL(state,
		oauth2.AccessTypeOffline, // request a refresh token
		oauth2.ApprovalForce,     // force consent so a refresh token is always issued
		oauth2.S256ChallengeOption(verifier),
	)

	resultCh := make(chan callbackResult, 1)

	srv := &http.Server{
		Handler:           callbackHandler(state, resultCh),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	go func() { _ = srv.Serve(ln) }()

	defer func() { _ = srv.Close() }() // ephemeral loopback server; no graceful drain needed

	fmt.Fprintln(w, "Opening your browser to authorize read-only access to Google Tasks…")
	fmt.Fprintf(w, "If it does not open automatically, visit this URL:\n\n%s\n\n", authURL)

	if browserErr := open(ctx, authURL); browserErr != nil {
		fmt.Fprintln(w, "(Could not open a browser automatically — please open the URL above manually.)")
	}

	fmt.Fprintln(w, "Waiting for authorization…")

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resultCh:
		if res.err != nil {
			return nil, res.err
		}

		tok, err := local.Exchange(ctx, res.code, oauth2.VerifierOption(verifier))
		if err != nil {
			return nil, fmt.Errorf("exchanging authorization code: %w", err)
		}

		return tok, nil
	}
}

// callbackHandler validates state, extracts the code (or error), serves a small
// "you can close this tab" page, and reports the result exactly once.
func callbackHandler(state string, resultCh chan<- callbackResult) http.HandlerFunc {
	var once sync.Once

	send := func(res callbackResult) { once.Do(func() { resultCh <- res }) }

	return func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(rw, r)
			return
		}

		q := r.URL.Query()

		if e := q.Get("error"); e != "" {
			writeBrowserPage(rw, "Authorization was denied. You can close this tab and return to the terminal.")
			send(callbackResult{err: fmt.Errorf("%w: %s", errAuthDenied, e)})

			return
		}

		if q.Get("state") != state {
			writeBrowserPage(rw, "Authorization failed (state mismatch). You can close this tab.")
			send(callbackResult{err: errStateMismatch})

			return
		}

		code := q.Get("code")
		if code == "" {
			writeBrowserPage(rw, "Authorization failed (no code returned). You can close this tab.")
			send(callbackResult{err: errNoAuthCode})

			return
		}

		writeBrowserPage(rw, "✓ Authorization complete. You can close this tab and return to the terminal.")
		send(callbackResult{code: code})
	}
}

func writeBrowserPage(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<!doctype html><html><head><meta charset=\"utf-8\"><title>gtasks</title></head>"+
		"<body style=\"font-family:system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 1rem\">"+
		"<h1>gtasks</h1><p>%s</p></body></html>", html.EscapeString(msg))
}

// loadToken reads a cached token from disk.
func loadToken(path string) (*oauth2.Token, error) {
	//nolint:gosec // G304: path is a user-supplied token cache path
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var tok oauth2.Token
	if err := json.NewDecoder(f).Decode(&tok); err != nil {
		return nil, fmt.Errorf("decoding token: %w", err)
	}

	return &tok, nil
}

// saveToken writes a token to disk atomically (temp file + rename) with 0600
// permissions, so an interrupted write never leaves a truncated token cache.
func saveToken(path string, tok *oauth2.Token) error {
	//nolint:gosec // G117: persisting the OAuth token to a 0600 cache is the intended behavior
	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding token: %w", err)
	}

	if err := atomicfile.Write(path, data, 0o600); err != nil {
		return fmt.Errorf("saving token %q: %w", path, err)
	}

	return nil
}
