package ui

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tasks "google.golang.org/api/tasks/v1"
)

func TestSelectIndex(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		n         int
		wantIndex int
		wantQuit  bool
		wantErr   error
	}{
		{name: "first", input: "1\n", n: 3, wantIndex: 0},
		{name: "last", input: "3\n", n: 3, wantIndex: 2},
		{name: "surrounding whitespace", input: "  2  \n", n: 3, wantIndex: 1},
		{name: "quit q", input: "q\n", n: 3, wantQuit: true},
		{name: "quit word", input: "quit\n", n: 3, wantQuit: true},
		{name: "quit uppercase", input: "Q\n", n: 3, wantQuit: true},
		{name: "retry then valid", input: "0\n9\nfoo\n2\n", n: 3, wantIndex: 1},
		{name: "eof no input", input: "", n: 3, wantErr: io.EOF},
		{name: "eof after invalid", input: "5", n: 3, wantErr: io.EOF},
		{name: "valid without newline", input: "2", n: 3, wantIndex: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var w bytes.Buffer

			idx, quit, err := SelectIndex(&w, strings.NewReader(tc.input), "pick: ", tc.n)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantQuit, quit)

			if !quit {
				assert.Equal(t, tc.wantIndex, idx)
			}
		})
	}
}

func TestSelectIndexReprompts(t *testing.T) {
	var w bytes.Buffer

	// "0" is out of range, then "2" is valid.
	_, _, err := SelectIndex(&w, strings.NewReader("0\n2\n"), "pick: ", 3)
	require.NoError(t, err)

	out := w.String()
	assert.Equal(t, 2, strings.Count(out, "pick: "))
	assert.Contains(t, out, "between 1 and 3")
}

func TestConfirm(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "y", input: "y\n", want: true},
		{name: "uppercase Y", input: "Y\n", want: true},
		{name: "yes", input: "yes\n", want: true},
		{name: "n", input: "n\n", want: false},
		{name: "blank line", input: "\n", want: false},
		{name: "eof", input: "", want: false},
		{name: "nonsense", input: "nonsense\n", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var w bytes.Buffer

			got, err := Confirm(&w, strings.NewReader(tc.input), "again?")
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Contains(t, w.String(), "again?")
		})
	}
}

func TestRenderLists(t *testing.T) {
	lists := []*tasks.TaskList{
		{Title: "My Tasks", Updated: "2026-08-01T12:00:00.000Z", Id: "abc"},
		{Title: "A much longer list title", Updated: "not-a-date", Id: "xyz"},
		{Title: "No date", Updated: "", Id: "id3"},
	}

	var w bytes.Buffer
	RenderLists(&w, lists)
	out := w.String()

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 4) // header + 3 rows

	assert.True(t, strings.HasPrefix(lines[0], "#"))
	assert.Contains(t, lines[0], "TITLE")
	assert.Contains(t, lines[0], "UPDATED")
	assert.Contains(t, lines[0], "ID")

	// Numbered 1..N.
	assert.True(t, strings.HasPrefix(lines[1], "1"))
	assert.True(t, strings.HasPrefix(lines[2], "2"))
	assert.True(t, strings.HasPrefix(lines[3], "3"))

	// RFC3339 rendered as a plain date.
	assert.Contains(t, lines[1], "2026-08-01")
	// Unparseable date passes through unchanged.
	assert.Contains(t, lines[2], "not-a-date")
	// Empty date shows a dash.
	assert.Contains(t, lines[3], "-")

	// tabwriter should pad the title column so IDs line up across rows.
	col1 := strings.Index(lines[1], "abc")

	col2 := strings.Index(lines[2], "xyz")
	assert.Equal(t, col1, col2)
}

func TestFormatDate(t *testing.T) {
	cases := map[string]string{
		"":                         "-",
		"2026-08-01T12:00:00.000Z": "2026-08-01",
		"garbage":                  "garbage",
	}

	for in, want := range cases {
		name := in
		if name == "" {
			name = "empty"
		}

		t.Run(name, func(t *testing.T) {
			assert.Equal(t, want, formatDate(in))
		})
	}
}

// TestIsInteractive exercises IsInteractive with plain files, which are never
// terminals — so the result is deterministic in CI and local runs alike.
func TestIsInteractive(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "notatty")
	require.NoError(t, err)

	defer func() { _ = f.Close() }()

	assert.False(t, IsInteractive(f, f))
}

// TestSelectIndexSharedReader verifies that two prompts sharing one bufio
// reader do not lose buffered input between them.
func TestSelectIndexSharedReader(t *testing.T) {
	var w bytes.Buffer

	shared := asReader(strings.NewReader("2\ny\n"))

	idx, quit, err := SelectIndex(&w, shared, "pick: ", 3)
	require.NoError(t, err)
	assert.False(t, quit)
	assert.Equal(t, 1, idx)

	again, err := Confirm(&w, shared, "again?")
	require.NoError(t, err)
	assert.True(t, again)
}
