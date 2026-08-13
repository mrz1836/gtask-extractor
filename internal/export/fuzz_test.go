package export

import (
	"path/filepath"
	"strings"
	"testing"

	tasks "google.golang.org/api/tasks/v1"
)

// FuzzFilename checks that the output filename derived from arbitrary, untrusted
// task-list titles and IDs is always a single, filesystem-safe path component.
// Task titles come straight from Google and can contain anything (Unicode,
// slashes, "..", NUL, very long strings), so a weak slug could otherwise let a
// title escape the output directory.
func FuzzFilename(f *testing.F) {
	seeds := []struct{ title, id string }{
		{"Sandbox", "MDEyMzQ1"},
		{"", ""},
		{"../../etc/passwd", "../../etc"},
		{"My / Weird \\ Title..", "id/slash:colon"},
		{"日本語タスク ☕", "！！！"},
		{strings.Repeat("A", 500), strings.Repeat("b", 500)},
		{"\x00\x01\x02\ncontrol\ttabs", "\r\n"},
		{".", ".."},
	}
	for _, s := range seeds {
		f.Add(s.title, s.id)
	}

	f.Fuzz(func(t *testing.T, title, id string) {
		name := filename(&tasks.TaskList{Title: title, Id: id}, fixedNow())

		if !strings.HasSuffix(name, ".json") {
			t.Fatalf("filename %q does not end in .json (title=%q id=%q)", name, title, id)
		}

		if len(name) <= len(".json") {
			t.Fatalf("filename %q has no name before the extension", name)
		}

		if strings.ContainsAny(name, `/\`) {
			t.Fatalf("filename %q contains a path separator", name)
		}

		if strings.Contains(name, "..") {
			t.Fatalf("filename %q contains %q", name, "..")
		}

		if name != filepath.Base(name) {
			t.Fatalf("filename %q is not a single path component", name)
		}
		// Joining into an output directory must not escape it.
		if dir := filepath.Dir(filepath.Join("output", name)); dir != "output" {
			t.Fatalf("filename %q escapes its directory: dir=%q", name, dir)
		}
		// Deterministic for the same inputs.
		if again := filename(&tasks.TaskList{Title: title, Id: id}, fixedNow()); again != name {
			t.Fatalf("filename not deterministic: %q vs %q", name, again)
		}
	})
}
