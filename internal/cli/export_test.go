package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tasks "google.golang.org/api/tasks/v1"

	"github.com/mrz1836/gtask-extractor/internal/export"
)

// fakeExportLister satisfies export.Lister without any network.
type fakeExportLister struct {
	tasks []*tasks.Task
	err   error
}

func (f fakeExportLister) ListTasks(_ context.Context, _ string) ([]*tasks.Task, error) {
	return f.tasks, f.err
}

func sampleLists() []*tasks.TaskList {
	return []*tasks.TaskList{
		{Id: "a", Title: "Alpha"},
		{Id: "b", Title: "Bravo"},
		{Id: "c", Title: "Charlie"},
	}
}

func TestResolveTargetsAll(t *testing.T) {
	got, err := resolveTargets(sampleLists(), nil, true)
	require.NoError(t, err)

	assert.Equal(t, "a,b,c", strings.Join(ids(got), ","))
}

func TestResolveTargetsByIDPreservesRequestOrder(t *testing.T) {
	got, err := resolveTargets(sampleLists(), []export.ListID{"c", "a"}, false)
	require.NoError(t, err)

	assert.Equal(t, []string{"c", "a"}, ids(got))
}

func TestResolveTargetsUnknownID(t *testing.T) {
	_, err := resolveTargets(sampleLists(), []export.ListID{"a", "zzz"}, false)
	require.ErrorIs(t, err, errListNotFound)

	assert.Contains(t, err.Error(), "zzz")
}

func ids(lists []*tasks.TaskList) []string {
	out := make([]string, len(lists))
	for i, l := range lists {
		out[i] = l.Id
	}

	return out
}

func TestExportListWritesFileAndSummary(t *testing.T) {
	dir := t.TempDir()
	opts := &options{outputDir: dir}
	fl := fakeExportLister{tasks: []*tasks.Task{
		{Id: "t1", Title: "one", Status: "needsAction"},
		{Id: "t2", Title: "two", Status: "completed"},
	}}

	var out, errW bytes.Buffer

	list := &tasks.TaskList{Id: "L1", Title: "Work"}
	require.NoError(t, exportList(context.Background(), fl, list, opts, &out, &errW))

	assert.Contains(t, out.String(), `✓ Exported 2 tasks from "Work"`)
	assert.Contains(t, errW.String(), "active: 1")
	assert.Contains(t, errW.String(), "completed: 1")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	assert.True(t, strings.HasSuffix(entries[0].Name(), ".json"))
	assert.True(t, strings.HasPrefix(entries[0].Name(), "work-L1-"))
}

func TestRunExportUsageGuards(t *testing.T) {
	newCmd := func() *cobra.Command {
		c := &cobra.Command{}
		c.SetContext(context.Background())
		c.SetOut(&bytes.Buffer{})
		c.SetErr(&bytes.Buffer{})

		return c
	}

	// Neither --list nor --all.
	require.ErrorIs(t, runExport(newCmd(), &options{}, nil, false), errNoTarget)
	// Both --list and --all.
	assert.ErrorIs(t, runExport(newCmd(), &options{}, []string{"a"}, true), errListAndAll)
}

func TestExportCommandWiring(t *testing.T) {
	// The export subcommand is registered and carries its flags.
	root := newRootCmd(BuildInfo{})

	var found *cobra.Command

	for _, c := range root.Commands() {
		if c.Name() == "export" {
			found = c
		}
	}

	require.NotNil(t, found)

	assert.NotNil(t, found.Flags().Lookup("list"))
	assert.NotNil(t, found.Flags().Lookup("all"))
}
