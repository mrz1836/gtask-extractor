package tasksclient

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	// option.WithHTTPClient makes NewService use the client verbatim, so this
	// constructs a fully wired Client without any network or ADC discovery.
	c, err := New(context.Background(), http.DefaultClient, "gtask-extractor/test")
	require.NoError(t, err)
	require.NotNil(t, c)
	require.NotNil(t, c.svc)

	assert.Equal(t, maxPages, c.maxPages)
}
