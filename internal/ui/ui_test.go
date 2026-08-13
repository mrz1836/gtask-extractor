package ui

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

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
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			if quit != tc.wantQuit {
				t.Errorf("quit = %v, want %v", quit, tc.wantQuit)
			}

			if !quit && idx != tc.wantIndex {
				t.Errorf("index = %d, want %d", idx, tc.wantIndex)
			}
		})
	}
}

func TestSelectIndexReprompts(t *testing.T) {
	var w bytes.Buffer
	// "0" is out of range, then "2" is valid.
	if _, _, err := SelectIndex(&w, strings.NewReader("0\n2\n"), "pick: ", 3); err != nil {
		t.Fatal(err)
	}

	out := w.String()
	if strings.Count(out, "pick: ") != 2 {
		t.Errorf("expected the prompt to be shown twice, got:\n%s", out)
	}

	if !strings.Contains(out, "between 1 and 3") {
		t.Errorf("expected a range hint, got:\n%s", out)
	}
}

func TestConfirm(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"n\n", false},
		{"\n", false},
		{"", false}, // EOF
		{"nonsense\n", false},
	}
	for _, tc := range cases {
		var w bytes.Buffer

		got, err := Confirm(&w, strings.NewReader(tc.input), "again?")
		if err != nil {
			t.Fatalf("Confirm(%q): %v", tc.input, err)
		}

		if got != tc.want {
			t.Errorf("Confirm(%q) = %v, want %v", tc.input, got, tc.want)
		}

		if !strings.Contains(w.String(), "again?") {
			t.Errorf("prompt not written for %q", tc.input)
		}
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
	if len(lines) != 4 { // header + 3 rows
		t.Fatalf("expected 4 lines, got %d:\n%s", len(lines), out)
	}

	if !strings.HasPrefix(lines[0], "#") || !strings.Contains(lines[0], "TITLE") ||
		!strings.Contains(lines[0], "UPDATED") || !strings.Contains(lines[0], "ID") {
		t.Errorf("header line = %q", lines[0])
	}
	// Numbered 1..N.
	if !strings.HasPrefix(lines[1], "1") || !strings.HasPrefix(lines[2], "2") || !strings.HasPrefix(lines[3], "3") {
		t.Errorf("rows not numbered 1..3:\n%s", out)
	}
	// RFC3339 rendered as a plain date.
	if !strings.Contains(lines[1], "2026-08-01") {
		t.Errorf("row 1 should show 2026-08-01: %q", lines[1])
	}
	// Unparseable date passes through unchanged.
	if !strings.Contains(lines[2], "not-a-date") {
		t.Errorf("row 2 should pass through the raw updated value: %q", lines[2])
	}
	// Empty date shows a dash.
	if !strings.Contains(lines[3], "-") {
		t.Errorf("row 3 should show a dash for empty date: %q", lines[3])
	}

	// tabwriter should pad the title column so IDs line up across rows.
	col1 := strings.Index(lines[1], "abc")

	col2 := strings.Index(lines[2], "xyz")
	if col1 != col2 {
		t.Errorf("ID column not aligned: %d vs %d\n%s", col1, col2, out)
	}
}

func TestFormatDate(t *testing.T) {
	cases := map[string]string{
		"":                         "-",
		"2026-08-01T12:00:00.000Z": "2026-08-01",
		"garbage":                  "garbage",
	}
	for in, want := range cases {
		if got := formatDate(in); got != want {
			t.Errorf("formatDate(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestIsInteractive exercises IsInteractive with plain files, which are never
// terminals — so the result is deterministic in CI and local runs alike.
func TestIsInteractive(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "notatty")
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = f.Close() }()

	if IsInteractive(f, f) {
		t.Error("a regular file should not be reported as an interactive terminal")
	}
}

// TestSelectIndexSharedReader verifies that two prompts sharing one bufio
// reader do not lose buffered input between them.
func TestSelectIndexSharedReader(t *testing.T) {
	var w bytes.Buffer

	shared := asReader(strings.NewReader("2\ny\n"))

	idx, quit, err := SelectIndex(&w, shared, "pick: ", 3)
	if err != nil || quit || idx != 1 {
		t.Fatalf("SelectIndex = (%d,%v,%v)", idx, quit, err)
	}

	again, err := Confirm(&w, shared, "again?")
	if err != nil {
		t.Fatal(err)
	}

	if !again {
		t.Errorf("second prompt lost buffered input; Confirm = false, want true")
	}
}
