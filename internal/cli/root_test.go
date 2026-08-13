package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"
)

func TestCodedError(t *testing.T) {
	base := errors.New("boom")
	err := coded(exitAPI, base)

	ce, ok := errors.AsType[*codedError](err)
	require.True(t, ok)

	assert.Equal(t, exitAPI, ce.code)
	require.ErrorIs(t, err, base)
	assert.Equal(t, "boom", err.Error())
}

func TestIsAPIError(t *testing.T) {
	assert.True(t, isAPIError(&googleapi.Error{Code: 500}))
	assert.False(t, isAPIError(errors.New("plain")))
}

func TestFriendlyAPIError(t *testing.T) {
	cases := []struct {
		name    string
		code    int
		wantSub string
	}{
		{"http 401", 401, "re-authorize"},
		{"http 403", 403, "Tasks API is enabled"},
	}

	const tokenPath = "token.json"

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := friendlyAPIError(&googleapi.Error{Code: c.code, Message: "m"}, tokenPath)
			assert.Contains(t, err.Error(), c.wantSub)
		})
	}

	// A non-API error is passed through unchanged.
	base := errors.New("plain")
	require.ErrorIs(t, friendlyAPIError(base, tokenPath), base)

	// An API error with an unmapped code is passed through.
	other := &googleapi.Error{Code: 500}
	require.ErrorIs(t, friendlyAPIError(other, tokenPath), error(other))
}

func TestVlogf(t *testing.T) {
	var buf bytes.Buffer

	vlogf(&buf, false, "hidden %d\n", 1)

	assert.Equal(t, 0, buf.Len())

	vlogf(&buf, true, "shown %d\n", 2)

	assert.Contains(t, buf.String(), "shown 2")
}

func TestVersionCommand(t *testing.T) {
	var buf bytes.Buffer

	root := newRootCmd(BuildInfo{})
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"version"})

	require.NoError(t, root.Execute())

	assert.Contains(t, buf.String(), "gtask-extractor")
}
