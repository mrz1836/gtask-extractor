package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"
	tasks "google.golang.org/api/tasks/v1"

	"github.com/mrz1836/gtask-extractor/internal/export"
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
	require.True(t, ok)

	return ce.code
}

func oneTask() []*tasks.Task { return []*tasks.Task{{Id: "t1", Status: "needsAction"}} }

// --- runExportWith (non-interactive core) ---

func TestRunExportWithMultiIDInOrder(t *testing.T) {
	dir := t.TempDir()
	fc := &fakeClient{lists: sampleLists(), tasks: oneTask()}

	var out bytes.Buffer
	require.NoError(t, runExportWith(context.Background(), fc, &options{outputDir: dir}, []export.ListID{"c", "a"}, false, &out, io.Discard))

	entries, _ := os.ReadDir(dir)
	require.Len(t, entries, 2)

	// Request order (Charlie before Alpha) must be preserved in the output.
	so := out.String()
	ci, ai := strings.Index(so, "Charlie"), strings.Index(so, "Alpha")
	assert.True(t, ci >= 0 && ai >= 0 && ci <= ai)
}

func TestRunExportWithAll(t *testing.T) {
	dir := t.TempDir()
	fc := &fakeClient{lists: sampleLists(), tasks: oneTask()}

	require.NoError(t, runExportWith(context.Background(), fc, &options{outputDir: dir}, nil, true, io.Discard, io.Discard))

	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 3)
}

func TestRunExportWithListErrorIsAPI(t *testing.T) {
	fc := &fakeClient{listsErr: &googleapi.Error{Code: 500}}

	err := runExportWith(context.Background(), fc, &options{outputDir: t.TempDir()}, nil, true, io.Discard, io.Discard)
	assert.Equal(t, exitAPI, exitCodeOf(t, err))
}

func TestRunExportWithUnknownIDIsUsage(t *testing.T) {
	fc := &fakeClient{lists: sampleLists()}

	err := runExportWith(context.Background(), fc, &options{outputDir: t.TempDir()}, []export.ListID{"zzz"}, false, io.Discard, io.Discard)
	assert.Equal(t, exitUsage, exitCodeOf(t, err))
	assert.ErrorIs(t, err, errListNotFound)
}

func TestRunExportWithExportFailureIsOutput(t *testing.T) {
	fc := &fakeClient{lists: sampleLists(), tasksErr: errors.New("boom")}

	err := runExportWith(context.Background(), fc, &options{outputDir: t.TempDir()}, []export.ListID{"a"}, false, io.Discard, io.Discard)
	assert.Equal(t, exitOutput, exitCodeOf(t, err))
}

// --- exportOnce (interactive iteration) ---

func TestExportOnceEmptyAccount(t *testing.T) {
	fc := &fakeClient{lists: nil}

	var errW bytes.Buffer

	more, err := exportOnce(context.Background(), &options{outputDir: t.TempDir()}, fc, strings.NewReader(""), io.Discard, &errW)
	require.NoError(t, err)
	require.False(t, more)

	assert.Contains(t, errW.String(), "No task lists")
}

func TestExportOnceListErrorIsAPI(t *testing.T) {
	fc := &fakeClient{listsErr: &googleapi.Error{Code: 403}}

	_, err := exportOnce(context.Background(), &options{outputDir: t.TempDir()}, fc, strings.NewReader(""), io.Discard, io.Discard)
	assert.Equal(t, exitAPI, exitCodeOf(t, err))
}

func TestExportOnceSelectThenStop(t *testing.T) {
	dir := t.TempDir()
	fc := &fakeClient{lists: sampleLists(), tasks: oneTask()}

	more, err := exportOnce(context.Background(), &options{outputDir: dir}, fc,
		bufio.NewReader(strings.NewReader("1\nn\n")), io.Discard, io.Discard)
	require.NoError(t, err)
	assert.False(t, more)

	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 1)
}

func TestExportOnceSelectThenContinue(t *testing.T) {
	fc := &fakeClient{lists: sampleLists(), tasks: oneTask()}

	more, err := exportOnce(context.Background(), &options{outputDir: t.TempDir()}, fc,
		bufio.NewReader(strings.NewReader("1\ny\n")), io.Discard, io.Discard)
	require.NoError(t, err)
	assert.True(t, more)
}

func TestExportOnceQuitAndEOF(t *testing.T) {
	for _, in := range []string{"q\n", ""} {
		fc := &fakeClient{lists: sampleLists()}

		more, err := exportOnce(context.Background(), &options{outputDir: t.TempDir()}, fc,
			bufio.NewReader(strings.NewReader(in)), io.Discard, io.Discard)
		require.NoError(t, err)
		assert.False(t, more)
	}
}

// --- exportList failure mapping ---

func TestExportListAPIErrorIsAPI(t *testing.T) {
	fl := fakeExportLister{err: &googleapi.Error{Code: 403}}

	err := exportList(context.Background(), fl, &tasks.TaskList{Id: "L", Title: "X"},
		&options{outputDir: t.TempDir()}, io.Discard, io.Discard)
	assert.Equal(t, exitAPI, exitCodeOf(t, err))
}

func TestExportListWriteErrorIsOutput(t *testing.T) {
	// Point the output dir at a regular file so MkdirAll fails.
	f, err := os.CreateTemp(t.TempDir(), "notadir")
	require.NoError(t, err)

	_ = f.Close()

	fl := fakeExportLister{tasks: oneTask()}

	err = exportList(context.Background(), fl, &tasks.TaskList{Id: "L", Title: "X"},
		&options{outputDir: f.Name()}, io.Discard, io.Discard)
	assert.Equal(t, exitOutput, exitCodeOf(t, err))
}
