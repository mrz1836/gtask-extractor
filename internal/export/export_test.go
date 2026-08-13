package export

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tasks "google.golang.org/api/tasks/v1"
)

var update = flag.Bool("update", false, "update golden files")

// fakeLister is an in-memory Lister; no network involved.
type fakeLister struct {
	tasks []*tasks.Task
	err   error
	gotID string
}

func (f *fakeLister) ListTasks(_ context.Context, listID string) ([]*tasks.Task, error) {
	f.gotID = listID
	return f.tasks, f.err
}

// sampleList / sampleTasks exercise every shape the exporter must preserve:
// top-level + subtask, completed + never-completed, deleted, hidden, assigned
// from a Doc (drive) and from a Space, links, and HTML-ish text.
func sampleList() *tasks.TaskList {
	return &tasks.TaskList{
		Kind:     "tasks#taskList",
		Id:       "MDEyMzQ1",
		Etag:     `"etag-list"`,
		Title:    "Sandbox",
		Updated:  "2026-08-01T12:00:00.000Z",
		SelfLink: "https://tasks.googleapis.com/tasks/v1/users/@me/lists/MDEyMzQ1",
	}
}

func sampleTasks() []*tasks.Task {
	return []*tasks.Task{
		{
			Kind:        "tasks#task",
			Id:          "t1",
			Etag:        `"etag1"`,
			Title:       "Buy groceries",
			Notes:       "milk & eggs <important>",
			Status:      "needsAction",
			Updated:     "2026-08-02T10:00:00.000Z",
			SelfLink:    "https://tasks.googleapis.com/tasks/v1/lists/MDEyMzQ1/tasks/t1",
			Position:    "00000000000000000000",
			Due:         "2026-08-10T00:00:00.000Z",
			Links:       []*tasks.TaskLinks{{Type: "email", Description: "Related email", Link: "https://mail.google.com/x"}},
			WebViewLink: "https://tasks.google.com/task/t1",
		},
		{
			Kind:        "tasks#task",
			Id:          "t2",
			Etag:        `"etag2"`,
			Title:       "Submit report",
			Status:      "completed",
			Updated:     "2026-08-05T09:30:00.000Z",
			SelfLink:    "https://tasks.googleapis.com/tasks/v1/lists/MDEyMzQ1/tasks/t2",
			Position:    "00000000000000000001",
			Completed:   new("2026-08-05T09:30:00.000Z"),
			WebViewLink: "https://tasks.google.com/task/t2",
		},
		{
			Kind:        "tasks#task",
			Id:          "t3",
			Etag:        `"etag3"`,
			Title:       "Pick up dry cleaning",
			Status:      "needsAction",
			Updated:     "2026-08-02T10:05:00.000Z",
			SelfLink:    "https://tasks.googleapis.com/tasks/v1/lists/MDEyMzQ1/tasks/t3",
			Parent:      "t1",
			Position:    "00000000000000000000",
			WebViewLink: "https://tasks.google.com/task/t3",
		},
		{
			Kind:      "tasks#task",
			Id:        "t4",
			Etag:      `"etag4"`,
			Title:     "Old cleared task",
			Status:    "completed",
			Updated:   "2026-07-01T08:00:00.000Z",
			SelfLink:  "https://tasks.googleapis.com/tasks/v1/lists/MDEyMzQ1/tasks/t4",
			Position:  "00000000000000000002",
			Completed: new("2026-07-01T08:00:00.000Z"),
			Hidden:    true,
		},
		{
			Kind:     "tasks#task",
			Id:       "t5",
			Etag:     `"etag5"`,
			Title:    "Removed task",
			Status:   "needsAction",
			Updated:  "2026-07-15T08:00:00.000Z",
			SelfLink: "https://tasks.googleapis.com/tasks/v1/lists/MDEyMzQ1/tasks/t5",
			Position: "00000000000000000003",
			Deleted:  true,
		},
		{
			Kind:     "tasks#task",
			Id:       "t6",
			Etag:     `"etag6"`,
			Title:    "Assigned from a Doc",
			Status:   "needsAction",
			Updated:  "2026-08-06T11:00:00.000Z",
			SelfLink: "https://tasks.googleapis.com/tasks/v1/lists/MDEyMzQ1/tasks/t6",
			Position: "00000000000000000004",
			AssignmentInfo: &tasks.AssignmentInfo{
				LinkToTask:        "https://docs.google.com/document/d/abc/edit",
				SurfaceType:       "DOCUMENT",
				DriveResourceInfo: &tasks.DriveResourceInfo{DriveFileId: "abc123", ResourceKey: "rk-1"},
			},
		},
		{
			Kind:     "tasks#task",
			Id:       "t7",
			Etag:     `"etag7"`,
			Title:    "Assigned from a Space",
			Status:   "needsAction",
			Updated:  "2026-08-06T12:00:00.000Z",
			SelfLink: "https://tasks.googleapis.com/tasks/v1/lists/MDEyMzQ1/tasks/t7",
			Position: "00000000000000000005",
			AssignmentInfo: &tasks.AssignmentInfo{
				LinkToTask:  "https://chat.google.com/room/xyz",
				SurfaceType: "SPACE",
				SpaceInfo:   &tasks.SpaceInfo{Space: "spaces/AAAA"},
			},
		},
	}
}

func fixedNow() time.Time { return time.Date(2026, 8, 12, 15, 4, 5, 0, time.UTC) }

func TestRunGolden(t *testing.T) {
	dir := t.TempDir()
	fl := &fakeLister{tasks: sampleTasks()}

	path, counts, err := Run(context.Background(), fl, sampleList(), Options{
		OutDir:      dir,
		ToolVersion: "test",
		Now:         fixedNow,
	})
	require.NoError(t, err)

	assert.Equal(t, "MDEyMzQ1", fl.gotID)
	assert.Equal(t, "sandbox-MDEyMzQ1-2026-08-12.json", filepath.Base(path))

	got, err := os.ReadFile(path)
	require.NoError(t, err)

	// The fixture uses a .golden extension (not .json) so repo-wide JSON
	// formatters (e.g. `magex format:fix`) never rewrite this byte-exact file.
	goldenPath := filepath.Join("testdata", "export.golden")
	if *update {
		if writeErr := os.WriteFile(goldenPath, got, 0o644); writeErr != nil {
			t.Fatalf("updating golden: %v", writeErr)
		}
	}

	want, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "reading golden (run `go test -run TestRunGolden -update`)")

	assert.Equal(t, want, got)

	// The file must end with a single trailing newline.
	assert.True(t, bytes.HasSuffix(got, []byte("}\n")))

	// HTML must not be escaped.
	assert.Contains(t, string(got), "milk & eggs <important>")

	wantCounts := Counts{Total: 7, NeedsAction: 5, Completed: 2, Deleted: 1, Hidden: 1, Assigned: 2, Subtasks: 1, TopLevel: 6}
	assert.Equal(t, wantCounts, counts)
}

// TestRunPreservesEveryField decodes the output and asserts the fields the raw
// API type would have dropped via omitempty are all present.
func TestRunPreservesEveryField(t *testing.T) {
	dir := t.TempDir()
	fl := &fakeLister{tasks: sampleTasks()}

	path, _, err := Run(context.Background(), fl, sampleList(), Options{OutDir: dir, ToolVersion: "test", Now: fixedNow})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var file File
	require.NoError(t, json.Unmarshal(data, &file))

	// A never-completed task keeps completed:null (present, not omitted).
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	rawTasks := raw["tasks"].([]any)

	t1 := rawTasks[0].(map[string]any)
	for _, key := range []string{"completed", "parent", "deleted", "hidden", "links", "assignmentInfo", "webViewLink", "position"} {
		assert.Contains(t, t1, key, "task[0] missing key %q (omitempty leak?)", key)
	}

	assert.Nil(t, t1["completed"], "task[0].completed should be null")
	assert.Equal(t, false, t1["deleted"], "task[0].deleted should be false")
	// Links must serialize as an array even when empty.
	t2 := rawTasks[1].(map[string]any)
	assert.IsType(t, []any{}, t2["links"], "task[1].links should be a JSON array")

	// A completed task keeps a concrete timestamp.
	require.NotNil(t, file.Tasks[1].Completed)
	assert.NotEmpty(t, *file.Tasks[1].Completed)
}

func TestRunEmptyList(t *testing.T) {
	dir := t.TempDir()
	fl := &fakeLister{tasks: nil}

	path, counts, err := Run(context.Background(), fl, sampleList(), Options{OutDir: dir, Now: fixedNow})
	require.NoError(t, err)

	assert.Equal(t, 0, counts.Total)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"tasks": []`)
}

func TestRunListerError(t *testing.T) {
	dir := t.TempDir()
	sentinel := errors.New("boom")
	fl := &fakeLister{err: sentinel}

	_, _, err := Run(context.Background(), fl, sampleList(), Options{OutDir: dir, Now: fixedNow})
	require.ErrorIs(t, err, sentinel)
}

func TestRunAtomicPermissions(t *testing.T) {
	dir := t.TempDir()
	fl := &fakeLister{tasks: sampleTasks()}

	path, _, err := Run(context.Background(), fl, sampleList(), Options{OutDir: dir, Now: fixedNow})
	require.NoError(t, err)

	info, err := os.Stat(path)
	require.NoError(t, err)

	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
	// No temp files should be left behind.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		assert.False(t, strings.HasPrefix(e.Name(), ".gtasks-"), "leftover temp file: %s", e.Name())
	}
}

func TestRunNilList(t *testing.T) {
	path, counts, err := Run(context.Background(), &fakeLister{}, nil, Options{OutDir: t.TempDir()})
	require.ErrorIs(t, err, errNilTaskList)

	assert.Empty(t, path)
	assert.Equal(t, Counts{}, counts)
}

func TestRunSkipsNilTasks(t *testing.T) {
	dir := t.TempDir()
	fl := &fakeLister{tasks: []*tasks.Task{
		{Id: "a", Status: "needsAction"},
		nil,
		{Id: "b", Status: "completed"},
	}}

	path, counts, err := Run(context.Background(), fl, sampleList(), Options{OutDir: dir, Now: fixedNow})
	require.NoError(t, err)

	assert.Equal(t, 2, counts.Total)

	var file File

	data, _ := os.ReadFile(path)
	require.NoError(t, json.Unmarshal(data, &file))

	assert.Len(t, file.Tasks, 2)
}

func TestFilenameSanitize(t *testing.T) {
	cases := []struct {
		name      string
		title, id string
		want      string
	}{
		{"simple", "Sandbox", "ABC", "sandbox-ABC-2026-08-12.json"},
		{"spaces and punctuation", "My Tasks!", "id1", "my-tasks-id1-2026-08-12.json"},
		{"path separators", "a/b\\c:d", "id2", "a-b-c-d-id2-2026-08-12.json"},
		{"leading and trailing spaces", "  Leading/Trailing  ", "id3", "leading-trailing-id3-2026-08-12.json"},
		{"collapses repeated separators", "Multiple   Spaces___here", "id4", "multiple-spaces-here-id4-2026-08-12.json"},
		// non-ASCII slug empties -> falls back to id.
		{"non-ascii falls back to id", "日本語", "id5", "id5-id5-2026-08-12.json"},
		{"empty title uses id", "", "OnlyID", "onlyid-OnlyID-2026-08-12.json"},
		{"very long title truncated", strings.Repeat("x", 200), "id6", strings.Repeat("x", 80) + "-id6-2026-08-12.json"},
		// Both title and id reduce to empty: fall back to a single clean segment
		// (no doubled dash).
		{"both empty -> tasklist", "日本語", "！！！", "tasklist-2026-08-12.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := filename(&tasks.TaskList{Title: c.title, Id: c.id}, fixedNow())
			assert.Equal(t, c.want, got)
		})
	}
}

// TestNoDroppedFields walks the json tags of every tasks.* type the exporter
// mirrors and fails if the envelope is missing any. A future library bump that
// adds a field will trip this test, forcing us to map the new field.
func TestNoDroppedFields(t *testing.T) {
	cases := []struct {
		name string
		api  reflect.Type
		env  reflect.Type
	}{
		{"Task", reflect.TypeFor[tasks.Task](), reflect.TypeFor[Task]()},
		{"TaskList", reflect.TypeFor[tasks.TaskList](), reflect.TypeFor[TaskList]()},
		{"TaskLinks", reflect.TypeFor[tasks.TaskLinks](), reflect.TypeFor[Link]()},
		{"AssignmentInfo", reflect.TypeFor[tasks.AssignmentInfo](), reflect.TypeFor[AssignmentInfo]()},
		{"DriveResourceInfo", reflect.TypeFor[tasks.DriveResourceInfo](), reflect.TypeFor[DriveResourceInfo]()},
		{"SpaceInfo", reflect.TypeFor[tasks.SpaceInfo](), reflect.TypeFor[SpaceInfo]()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			apiTags := jsonTags(c.api)

			envTags := jsonTags(c.env)
			for tag := range apiTags {
				assert.True(t, envTags[tag], "%s: API field %q is not represented in the export envelope (library bump added a field?)", c.name, tag)
			}
		})
	}
}

// jsonTags returns the set of json field names on a struct, skipping fields
// tagged "-" (ServerResponse, ForceSendFields, NullFields) and untagged ones.
func jsonTags(t reflect.Type) map[string]bool {
	tags := map[string]bool{}

	for f := range t.Fields() {
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}

		name, _, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}

		tags[name] = true
	}

	return tags
}
