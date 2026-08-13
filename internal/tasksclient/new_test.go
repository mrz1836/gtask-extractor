package tasksclient

import (
	"context"
	"net/http"
	"testing"
)

func TestNew(t *testing.T) {
	// option.WithHTTPClient makes NewService use the client verbatim, so this
	// constructs a fully wired Client without any network or ADC discovery.
	c, err := New(context.Background(), http.DefaultClient, "gtask-extractor/test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if c == nil || c.svc == nil {
		t.Fatal("New returned an incomplete client")
	}

	if c.maxPages != maxPages {
		t.Errorf("maxPages = %d, want %d", c.maxPages, maxPages)
	}
}
