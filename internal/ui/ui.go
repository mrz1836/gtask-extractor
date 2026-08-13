// Package ui implements the small interactive layer: a numbered task-list table
// and a stdin-driven picker. It deliberately avoids any TUI dependency — the
// whole thing is a few dozen lines of standard library.
package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"
	tasks "google.golang.org/api/tasks/v1"
)

// IsInteractive reports whether both in and out refer to a terminal. gtasks
// requires a TTY because its only UI is the interactive picker.
func IsInteractive(in, out *os.File) bool {
	return term.IsTerminal(int(in.Fd())) && term.IsTerminal(int(out.Fd()))
}

// RenderLists prints an aligned, numbered table of task lists to w.
func RenderLists(w io.Writer, lists []*tasks.TaskList) {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tTITLE\tUPDATED\tID")

	for i, l := range lists {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", i+1, l.Title, formatDate(l.Updated), l.Id)
	}

	_ = tw.Flush()
}

// formatDate renders an RFC-3339 timestamp as a plain date, leaving anything
// unparseable untouched.
func formatDate(s string) string {
	if s == "" {
		return "-"
	}

	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format("2006-01-02")
	}

	return s
}

// SelectIndex prompts for a 1-based selection in [1,n], reading lines from r and
// writing prompts/errors to w. It returns quit=true for "q"/"quit"/"exit", and
// io.EOF if input ends before a valid selection is made. Invalid entries are
// re-prompted.
func SelectIndex(w io.Writer, r io.Reader, prompt string, n int) (index int, quit bool, err error) {
	br := asReader(r)

	for {
		fmt.Fprint(w, prompt)

		line, readErr := br.ReadString('\n')
		trimmed := strings.TrimSpace(line)

		switch strings.ToLower(trimmed) {
		case "q", "quit", "exit":
			return 0, true, nil
		}

		if num, convErr := strconv.Atoi(trimmed); convErr == nil && num >= 1 && num <= n {
			return num - 1, false, nil
		}

		if readErr != nil {
			// Ran out of input without a valid selection.
			return 0, false, io.EOF
		}

		fmt.Fprintf(w, "Please enter a number between 1 and %d (or q to quit).\n", n)
	}
}

// Confirm asks a yes/no question, defaulting to no. EOF is treated as "no".
func Confirm(w io.Writer, r io.Reader, prompt string) (bool, error) {
	br := asReader(r)

	fmt.Fprintf(w, "%s [y/N]: ", prompt)

	line, err := br.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	}

	if err != nil && err != io.EOF {
		return false, err
	}

	return false, nil
}

// asReader returns r as a *bufio.Reader, reusing it if it already is one. This
// matters when several prompts share a single underlying stream (os.Stdin):
// wrapping it afresh each time could discard bytes a previous reader buffered.
func asReader(r io.Reader) *bufio.Reader {
	if br, ok := r.(*bufio.Reader); ok {
		return br
	}

	return bufio.NewReader(r)
}
