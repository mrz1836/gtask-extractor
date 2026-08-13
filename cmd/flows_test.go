package cmd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"google.golang.org/api/googleapi"
	tasks "google.golang.org/api/tasks/v1"
)

// fakeClient satisfies taskLister (and, via ListTasks, export.Lister) with no
// network. It backs the interactive and non-interactive flow tests.
type fakeClient struct {
	lists    []*tasks.TaskList
	listsErr error
	tasks    []*tasks.Task
	tasksErr error
}

func (f *fakeClient) ListTaskLists(context.Context) ([]*tasks.TaskList, error) {
	return f.lists, f.listsErr
}

func (f *fakeClient) ListTasks(context.Context, string) ([]*tasks.Task, error) {
	return f.tasks, f.tasksErr
}

func exitCodeOf(t *testing.T, err error) int {
	t.Helper()

	ce, ok := errors.AsType[*codedError](err)
	if !ok {
		t.Fatalf("error %v is not a *codedError", err)
	}

	return ce.code
}

func oneTask() []*tasks.Task { return []*tasks.Task{{Id: "t1", Status: "needsAction"}} }

// --- runExportWith (non-interactive core) ---

func TestRunExportWithMultiIDInOrder(t *testing.T) {
	dir := t.TempDir()
	fc := &fakeClient{lists: sampleLists(), tasks: oneTask()}

	var out bytes.Buffer
	if err := runExportWith(context.Background(), fc, &options{outputDir: dir}, []string{"c", "a"}, false, &out, io.Discard); err != nil {
		t.Fatal(err)
	}

	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Fatalf("wrote %d files, want 2", len(entries))
	}
	// Request order (Charlie before Alpha) must be preserved in the output.
	so := out.String()
	if ci, ai := strings.Index(so, "Charlie"), strings.Index(so, "Alpha"); ci < 0 || ai < 0 || ci > ai {
		t.Errorf("expected Charlie before Alpha:\n%s", so)
	}
}

func TestRunExportWithAll(t *testing.T) {
	dir := t.TempDir()
	fc := &fakeClient{lists: sampleLists(), tasks: oneTask()}

	if err := runExportWith(context.Background(), fc, &options{outputDir: dir}, nil, true, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}

	if entries, _ := os.ReadDir(dir); len(entries) != 3 {
		t.Errorf("wrote %d files, want 3 (--all)", len(entries))
	}
}

func TestRunExportWithListErrorIsAPI(t *testing.T) {
	fc := &fakeClient{listsErr: &googleapi.Error{Code: 500}}

	err := runExportWith(context.Background(), fc, &options{outputDir: t.TempDir()}, nil, true, io.Discard, io.Discard)
	if got := exitCodeOf(t, err); got != exitAPI {
		t.Errorf("code = %d, want exitAPI (%d)", got, exitAPI)
	}
}

func TestRunExportWithUnknownIDIsUsage(t *testing.T) {
	fc := &fakeClient{lists: sampleLists()}

	err := runExportWith(context.Background(), fc, &options{outputDir: t.TempDir()}, []string{"zzz"}, false, io.Discard, io.Discard)
	if got := exitCodeOf(t, err); got != exitUsage {
		t.Errorf("code = %d, want exitUsage (%d)", got, exitUsage)
	}

	if !errors.Is(err, errListNotFound) {
		t.Errorf("want errListNotFound, got %v", err)
	}
}

func TestRunExportWithExportFailureIsOutput(t *testing.T) {
	fc := &fakeClient{lists: sampleLists(), tasksErr: errors.New("boom")}

	err := runExportWith(context.Background(), fc, &options{outputDir: t.TempDir()}, []string{"a"}, false, io.Discard, io.Discard)
	if got := exitCodeOf(t, err); got != exitOutput {
		t.Errorf("code = %d, want exitOutput (%d)", got, exitOutput)
	}
}

// --- exportOnce (interactive iteration) ---

func TestExportOnceEmptyAccount(t *testing.T) {
	fc := &fakeClient{lists: nil}

	var errW bytes.Buffer

	more, err := exportOnce(context.Background(), &options{outputDir: t.TempDir()}, fc, strings.NewReader(""), io.Discard, &errW)
	if err != nil || more {
		t.Fatalf("more=%v err=%v, want false,nil", more, err)
	}

	if !strings.Contains(errW.String(), "No task lists") {
		t.Errorf("errW = %q", errW.String())
	}
}

func TestExportOnceListErrorIsAPI(t *testing.T) {
	fc := &fakeClient{listsErr: &googleapi.Error{Code: 403}}

	_, err := exportOnce(context.Background(), &options{outputDir: t.TempDir()}, fc, strings.NewReader(""), io.Discard, io.Discard)
	if got := exitCodeOf(t, err); got != exitAPI {
		t.Errorf("code = %d, want exitAPI", got)
	}
}

func TestExportOnceSelectThenStop(t *testing.T) {
	dir := t.TempDir()
	fc := &fakeClient{lists: sampleLists(), tasks: oneTask()}

	more, err := exportOnce(context.Background(), &options{outputDir: dir}, fc,
		bufio.NewReader(strings.NewReader("1\nn\n")), io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	if more {
		t.Errorf("more = true, want false (answered n)")
	}

	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("wrote %d files, want 1", len(entries))
	}
}

func TestExportOnceSelectThenContinue(t *testing.T) {
	fc := &fakeClient{lists: sampleLists(), tasks: oneTask()}

	more, err := exportOnce(context.Background(), &options{outputDir: t.TempDir()}, fc,
		bufio.NewReader(strings.NewReader("1\ny\n")), io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	if !more {
		t.Errorf("more = false, want true (answered y)")
	}
}

func TestExportOnceQuitAndEOF(t *testing.T) {
	for _, in := range []string{"q\n", ""} {
		fc := &fakeClient{lists: sampleLists()}

		more, err := exportOnce(context.Background(), &options{outputDir: t.TempDir()}, fc,
			bufio.NewReader(strings.NewReader(in)), io.Discard, io.Discard)
		if err != nil || more {
			t.Errorf("input %q: more=%v err=%v, want false,nil", in, more, err)
		}
	}
}

// --- exportList failure mapping ---

func TestExportListAPIErrorIsAPI(t *testing.T) {
	fl := fakeExportLister{err: &googleapi.Error{Code: 403}}

	err := exportList(context.Background(), fl, &tasks.TaskList{Id: "L", Title: "X"},
		&options{outputDir: t.TempDir()}, io.Discard, io.Discard)
	if got := exitCodeOf(t, err); got != exitAPI {
		t.Errorf("code = %d, want exitAPI", got)
	}
}

func TestExportListWriteErrorIsOutput(t *testing.T) {
	// Point the output dir at a regular file so MkdirAll fails.
	f, err := os.CreateTemp(t.TempDir(), "notadir")
	if err != nil {
		t.Fatal(err)
	}

	_ = f.Close()

	fl := fakeExportLister{tasks: oneTask()}

	err = exportList(context.Background(), fl, &tasks.TaskList{Id: "L", Title: "X"},
		&options{outputDir: f.Name()}, io.Discard, io.Discard)
	if got := exitCodeOf(t, err); got != exitOutput {
		t.Errorf("code = %d, want exitOutput", got)
	}
}
