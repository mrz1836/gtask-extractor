package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"google.golang.org/api/googleapi"
)

func TestCodedError(t *testing.T) {
	base := errors.New("boom")
	err := coded(exitAPI, base)

	ce, ok := errors.AsType[*codedError](err)
	if !ok {
		t.Fatalf("coded did not return a *codedError")
	}

	if ce.code != exitAPI {
		t.Errorf("code = %d, want %d", ce.code, exitAPI)
	}

	if !errors.Is(err, base) {
		t.Errorf("coded error should unwrap to the base error")
	}

	if err.Error() != "boom" {
		t.Errorf("Error() = %q, want %q", err.Error(), "boom")
	}
}

func TestIsAPIError(t *testing.T) {
	if !isAPIError(&googleapi.Error{Code: 500}) {
		t.Error("googleapi.Error should be recognized")
	}

	if isAPIError(errors.New("plain")) {
		t.Error("plain error should not be recognized as an API error")
	}
}

func TestFriendlyAPIError(t *testing.T) {
	cases := []struct {
		code    int
		wantSub string
	}{
		{401, "re-authorize"},
		{403, "Tasks API is enabled"},
	}

	const tokenPath = "token.json"
	for _, c := range cases {
		err := friendlyAPIError(&googleapi.Error{Code: c.code, Message: "m"}, tokenPath)
		if !strings.Contains(err.Error(), c.wantSub) {
			t.Errorf("code %d: %q does not contain %q", c.code, err.Error(), c.wantSub)
		}
	}

	// A non-API error is passed through unchanged.
	base := errors.New("plain")
	if got := friendlyAPIError(base, tokenPath); !errors.Is(got, base) {
		t.Errorf("friendlyAPIError(plain) = %v, want the same error", got)
	}
	// An API error with an unmapped code is passed through.
	other := &googleapi.Error{Code: 500}
	if got := friendlyAPIError(other, tokenPath); !errors.Is(got, error(other)) {
		t.Errorf("unmapped API error should be returned unchanged")
	}
}

func TestVlogf(t *testing.T) {
	var buf bytes.Buffer

	vlogf(&buf, false, "hidden %d\n", 1)

	if buf.Len() != 0 {
		t.Errorf("vlogf should be silent when verbose is off, got %q", buf.String())
	}

	vlogf(&buf, true, "shown %d\n", 2)

	if !strings.Contains(buf.String(), "shown 2") {
		t.Errorf("vlogf should write when verbose is on, got %q", buf.String())
	}
}

func TestVersionCommand(t *testing.T) {
	var buf bytes.Buffer

	root := newRootCmd(BuildInfo{})
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("version command: %v", err)
	}

	if !strings.Contains(buf.String(), "gtask-extractor") {
		t.Errorf("version output = %q, want it to contain 'gtask-extractor'", buf.String())
	}
}
