package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	tasks "google.golang.org/api/tasks/v1"
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
	if err != nil {
		t.Fatal(err)
	}

	if want := "a,b,c"; strings.Join(ids(got), ",") != want {
		t.Errorf("all: ids = %v, want %s in order", ids(got), want)
	}
}

func TestResolveTargetsByIDPreservesRequestOrder(t *testing.T) {
	got, err := resolveTargets(sampleLists(), []string{"c", "a"}, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 || got[0].Id != "c" || got[1].Id != "a" {
		t.Errorf("got %v, want [c a] in that order", ids(got))
	}
}

func TestResolveTargetsUnknownID(t *testing.T) {
	_, err := resolveTargets(sampleLists(), []string{"a", "zzz"}, false)
	if !errors.Is(err, errListNotFound) {
		t.Fatalf("err = %v, want errListNotFound", err)
	}

	if !strings.Contains(err.Error(), "zzz") {
		t.Errorf("err should name the missing ID, got %v", err)
	}
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
	if err := exportList(context.Background(), fl, list, opts, &out, &errW); err != nil {
		t.Fatalf("exportList: %v", err)
	}

	if !strings.Contains(out.String(), `✓ Exported 2 tasks from "Work"`) {
		t.Errorf("stdout = %q, want the export summary line", out.String())
	}

	if !strings.Contains(errW.String(), "active: 1") || !strings.Contains(errW.String(), "completed: 1") {
		t.Errorf("stderr = %q, want a counts breakdown", errW.String())
	}

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one output file, got %v (err=%v)", entries, err)
	}

	if !strings.HasSuffix(entries[0].Name(), ".json") || !strings.HasPrefix(entries[0].Name(), "work-L1-") {
		t.Errorf("unexpected filename %q", entries[0].Name())
	}
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
	if err := runExport(newCmd(), &options{}, nil, false); !errors.Is(err, errNoTarget) {
		t.Errorf("no target: err = %v, want errNoTarget", err)
	}
	// Both --list and --all.
	if err := runExport(newCmd(), &options{}, []string{"a"}, true); !errors.Is(err, errListAndAll) {
		t.Errorf("both: err = %v, want errListAndAll", err)
	}
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

	if found == nil {
		t.Fatal("export subcommand not registered")
	}

	if found.Flags().Lookup("list") == nil || found.Flags().Lookup("all") == nil {
		t.Error("export subcommand missing --list/--all flags")
	}
}
