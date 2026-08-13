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
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if fl.gotID != "MDEyMzQ1" {
		t.Errorf("ListTasks got list id %q, want MDEyMzQ1", fl.gotID)
	}

	if base := filepath.Base(path); base != "sandbox-MDEyMzQ1-2026-08-12.json" {
		t.Errorf("filename = %q, want sandbox-MDEyMzQ1-2026-08-12.json", base)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading export: %v", err)
	}

	// The fixture uses a .golden extension (not .json) so repo-wide JSON
	// formatters (e.g. `magex format:fix`) never rewrite this byte-exact file.
	goldenPath := filepath.Join("testdata", "export.golden")
	if *update {
		if writeErr := os.WriteFile(goldenPath, got, 0o644); writeErr != nil {
			t.Fatalf("updating golden: %v", writeErr)
		}
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden (run `go test -run TestRunGolden -update`): %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("export does not match golden.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	// The file must end with a single trailing newline.
	if !bytes.HasSuffix(got, []byte("}\n")) {
		t.Errorf("export should end with a trailing newline")
	}

	// HTML must not be escaped.
	if !bytes.Contains(got, []byte("milk & eggs <important>")) {
		t.Errorf("notes appear to be HTML-escaped; SetEscapeHTML(false) not honored")
	}

	wantCounts := Counts{Total: 7, NeedsAction: 5, Completed: 2, Deleted: 1, Hidden: 1, Assigned: 2, Subtasks: 1, TopLevel: 6}
	if counts != wantCounts {
		t.Errorf("counts = %+v, want %+v", counts, wantCounts)
	}
}

// TestRunPreservesEveryField decodes the output and asserts the fields the raw
// API type would have dropped via omitempty are all present.
func TestRunPreservesEveryField(t *testing.T) {
	dir := t.TempDir()
	fl := &fakeLister{tasks: sampleTasks()}

	path, _, err := Run(context.Background(), fl, sampleList(), Options{OutDir: dir, ToolVersion: "test", Now: fixedNow})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// A never-completed task keeps completed:null (present, not omitted).
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	rawTasks := raw["tasks"].([]any)

	t1 := rawTasks[0].(map[string]any)
	for _, key := range []string{"completed", "parent", "deleted", "hidden", "links", "assignmentInfo", "webViewLink", "position"} {
		if _, ok := t1[key]; !ok {
			t.Errorf("task[0] missing key %q (omitempty leak?)", key)
		}
	}

	if t1["completed"] != nil {
		t.Errorf("task[0].completed should be null, got %v", t1["completed"])
	}

	if t1["deleted"] != false {
		t.Errorf("task[0].deleted should be false, got %v", t1["deleted"])
	}
	// Links must serialize as an array even when empty.
	t2 := rawTasks[1].(map[string]any)
	if _, ok := t2["links"].([]any); !ok {
		t.Errorf("task[1].links should be a JSON array, got %T", t2["links"])
	}

	// A completed task keeps a concrete timestamp.
	if file.Tasks[1].Completed == nil || *file.Tasks[1].Completed == "" {
		t.Errorf("task[1].completed should be a timestamp")
	}
}

func TestRunEmptyList(t *testing.T) {
	dir := t.TempDir()
	fl := &fakeLister{tasks: nil}

	path, counts, err := Run(context.Background(), fl, sampleList(), Options{OutDir: dir, Now: fixedNow})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if counts.Total != 0 {
		t.Errorf("counts.Total = %d, want 0", counts.Total)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(data, []byte(`"tasks": []`)) {
		t.Errorf("empty export should contain `\"tasks\": []`, got:\n%s", data)
	}
}

func TestRunListerError(t *testing.T) {
	dir := t.TempDir()
	sentinel := errors.New("boom")
	fl := &fakeLister{err: sentinel}

	_, _, err := Run(context.Background(), fl, sampleList(), Options{OutDir: dir, Now: fixedNow})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Run error = %v, want wrapping sentinel", err)
	}
}

func TestRunAtomicPermissions(t *testing.T) {
	dir := t.TempDir()
	fl := &fakeLister{tasks: sampleTasks()}

	path, _, err := Run(context.Background(), fl, sampleList(), Options{OutDir: dir, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("output perm = %o, want 644", perm)
	}
	// No temp files should be left behind.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".gtasks-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

func TestRunNilList(t *testing.T) {
	path, counts, err := Run(context.Background(), &fakeLister{}, nil, Options{OutDir: t.TempDir()})
	if !errors.Is(err, errNilTaskList) {
		t.Fatalf("err = %v, want errNilTaskList", err)
	}

	if path != "" || counts != (Counts{}) {
		t.Errorf("expected empty path and zero counts, got path=%q counts=%+v", path, counts)
	}
}

func TestRunSkipsNilTasks(t *testing.T) {
	dir := t.TempDir()
	fl := &fakeLister{tasks: []*tasks.Task{
		{Id: "a", Status: "needsAction"},
		nil,
		{Id: "b", Status: "completed"},
	}}

	path, counts, err := Run(context.Background(), fl, sampleList(), Options{OutDir: dir, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}

	if counts.Total != 2 {
		t.Errorf("counts.Total = %d, want 2 (nil skipped)", counts.Total)
	}

	var file File

	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}

	if len(file.Tasks) != 2 {
		t.Errorf("wrote %d tasks, want 2 (nil skipped)", len(file.Tasks))
	}
}

func TestFilenameSanitize(t *testing.T) {
	cases := []struct {
		title, id string
		want      string
	}{
		{"Sandbox", "ABC", "sandbox-ABC-2026-08-12.json"},
		{"My Tasks!", "id1", "my-tasks-id1-2026-08-12.json"},
		{"a/b\\c:d", "id2", "a-b-c-d-id2-2026-08-12.json"},
		{"  Leading/Trailing  ", "id3", "leading-trailing-id3-2026-08-12.json"},
		{"Multiple   Spaces___here", "id4", "multiple-spaces-here-id4-2026-08-12.json"},
		{"日本語", "id5", "id5-id5-2026-08-12.json"}, // non-ASCII slug empties -> falls back to id
		{"", "OnlyID", "onlyid-OnlyID-2026-08-12.json"},
		{strings.Repeat("x", 200), "id6", strings.Repeat("x", 80) + "-id6-2026-08-12.json"},
		// Both title and id reduce to empty: fall back to a single clean segment
		// (no doubled dash).
		{"日本語", "！！！", "tasklist-2026-08-12.json"},
	}
	for _, c := range cases {
		got := filename(&tasks.TaskList{Title: c.title, Id: c.id}, fixedNow())
		if got != c.want {
			t.Errorf("filename(%q,%q) = %q, want %q", c.title, c.id, got, c.want)
		}
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
		apiTags := jsonTags(c.api)

		envTags := jsonTags(c.env)
		for tag := range apiTags {
			if !envTags[tag] {
				t.Errorf("%s: API field %q is not represented in the export envelope (library bump added a field?)", c.name, tag)
			}
		}
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
