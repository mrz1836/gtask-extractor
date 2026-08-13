// Package export builds and writes the JSON export document for a single task
// list. It owns its own envelope structs (see model.go) so that every field of
// every task is emitted verbatim, including the ones the Tasks API drops via
// `omitempty` (deleted:false, hidden:false, completed:null, empty strings).
package export

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tasks "google.golang.org/api/tasks/v1"
)

const (
	toolName      = "gtask-extractor"
	scopeReadonly = "https://www.googleapis.com/auth/tasks.readonly"

	// maxSlugLen caps the title-derived portion of the output filename.
	maxSlugLen = 80
)

// errNilTaskList is returned when Run is called without a task list.
var errNilTaskList = errors.New("nil task list")

// Lister is the subset of the tasks client that export depends on. Keeping it
// narrow lets tests supply a trivial fake.
type Lister interface {
	ListTasks(ctx context.Context, listID string) ([]*tasks.Task, error)
}

// Options configures a Run. Now is injectable so tests get deterministic output.
type Options struct {
	OutDir      string
	ToolVersion string
	Now         func() time.Time
}

// Run fetches every task in list, builds the export envelope, and atomically
// writes it to OutDir. It returns the written path and a count breakdown.
func Run(ctx context.Context, client Lister, list *tasks.TaskList, opts Options) (string, Counts, error) {
	if list == nil {
		return "", Counts{}, errNilTaskList
	}

	if opts.Now == nil {
		opts.Now = time.Now
	}

	if opts.ToolVersion == "" {
		opts.ToolVersion = "dev"
	}

	apiTasks, err := client.ListTasks(ctx, list.Id)
	if err != nil {
		return "", Counts{}, fmt.Errorf("fetching tasks for list %q: %w", list.Id, err)
	}

	file := buildFile(list, apiTasks, opts)

	path := filepath.Join(opts.OutDir, filename(list, opts.Now()))
	if err := writeAtomic(path, file); err != nil {
		return "", file.Export.Counts, err
	}

	return path, file.Export.Counts, nil
}

// buildFile assembles the export document and tallies the counts.
func buildFile(list *tasks.TaskList, apiTasks []*tasks.Task, opts Options) File {
	outTasks := make([]Task, 0, len(apiTasks))
	for _, t := range apiTasks {
		if t == nil {
			continue
		}

		outTasks = append(outTasks, fromAPITask(t))
	}

	return File{
		SchemaVersion: SchemaVersion,
		Export: Summary{
			GeneratedAt: opts.Now().UTC().Format(time.RFC3339),
			Tool:        toolName,
			ToolVersion: opts.ToolVersion,
			Scope:       scopeReadonly,
			ListID:      list.Id,
			ListTitle:   list.Title,
			Counts:      tallyCounts(apiTasks),
		},
		List:  fromAPITaskList(list),
		Tasks: outTasks,
	}
}

// tallyCounts summarizes a task slice. The buckets are not mutually exclusive.
func tallyCounts(apiTasks []*tasks.Task) Counts {
	var c Counts

	for _, t := range apiTasks {
		if t == nil {
			continue
		}

		c.Total++

		switch t.Status {
		case "completed":
			c.Completed++
		case "needsAction":
			c.NeedsAction++
		}

		if t.Deleted {
			c.Deleted++
		}

		if t.Hidden {
			c.Hidden++
		}

		if t.AssignmentInfo != nil {
			c.Assigned++
		}

		if t.Parent != "" {
			c.Subtasks++
		} else {
			c.TopLevel++
		}
	}

	return c
}

// marshal renders the export document as indented JSON with a trailing newline.
// HTML escaping is disabled so notes/links containing <, > or & stay readable.
func marshal(file File) ([]byte, error) {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	if err := enc.Encode(file); err != nil {
		return nil, fmt.Errorf("encoding export: %w", err)
	}

	return buf.Bytes(), nil
}

// writeAtomic writes the document to a temp file and renames it into place so a
// reader never observes a partially written file.
func writeAtomic(path string, file File) error {
	data, err := marshal(file)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if mkErr := os.MkdirAll(dir, 0o750); mkErr != nil {
		return fmt.Errorf("creating output directory %q: %w", dir, mkErr)
	}

	tmp, err := os.CreateTemp(dir, ".gtasks-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op after a successful rename

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing export: %w", err)
	}

	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setting output permissions: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("finalizing output %q: %w", path, err)
	}

	return nil
}

// filename derives output/<slug>-<listID>-<YYYY-MM-DD>.json, dropping any
// segment that reduces to empty so the result never contains a doubled dash.
func filename(list *tasks.TaskList, now time.Time) string {
	slug := slugify(list.Title)
	if slug == "" {
		slug = slugify(list.Id)
	}

	parts := make([]string, 0, 3)

	for _, p := range []string{slug, sanitizeID(list.Id)} {
		if p != "" {
			parts = append(parts, p)
		}
	}

	if len(parts) == 0 {
		parts = append(parts, "tasklist")
	}

	parts = append(parts, now.UTC().Format("2006-01-02"))

	return strings.Join(parts, "-") + ".json"
}

var (
	nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)
	nonIDChars   = regexp.MustCompile(`[^A-Za-z0-9_-]+`)
)

// slugify lowercases, replaces every run of non-alphanumeric characters with a
// single dash, trims dashes, and caps the length. Non-ASCII input that reduces
// to nothing yields "" (the caller then falls back to the list ID).
func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonSlugChars.ReplaceAllString(s, "-")

	s = strings.Trim(s, "-")
	if len(s) > maxSlugLen {
		s = strings.Trim(s[:maxSlugLen], "-")
	}

	return s
}

// sanitizeID keeps only filesystem-safe characters from a list ID.
func sanitizeID(s string) string {
	return nonIDChars.ReplaceAllString(s, "")
}
