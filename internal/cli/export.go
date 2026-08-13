package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	tasks "google.golang.org/api/tasks/v1"

	"github.com/mrz1836/gtask-extractor/internal/export"
)

// newExportCmd builds the non-interactive `export` subcommand. It exports one or
// more lists by ID (or all lists) straight to JSON — no prompts, no TTY — which
// makes it suitable for scripts, cron jobs, and re-pulling the same lists.
func newExportCmd(opts *options) *cobra.Command {
	var (
		listIDs []string
		all     bool
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export task lists non-interactively (by ID, or all)",
		Long: `Export one or more task lists straight to JSON without any prompts.

Examples:
  gtasks export --list <id>              export a single list
  gtasks export --list <id> --list <id>  export several lists
  gtasks export --all                    export every task list

Find list IDs by running gtasks with no arguments (the interactive picker
shows each list's ID), or with "gtasks export --all".

First run still opens a browser once to authorize read-only access; after that
the cached token is reused, so this command runs unattended.`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runExport(cmd, opts, listIDs, all)
		},
	}

	f := cmd.Flags()
	f.StringArrayVar(&listIDs, "list", nil, "task list ID to export (repeatable)")
	f.BoolVar(&all, "all", false, "export every task list")
	cmd.MarkFlagsMutuallyExclusive("list", "all")

	return cmd
}

// runExport validates the selection, authenticates, and hands off to the
// injectable core (runExportWith). Keeping auth here and the list/export logic
// in runExportWith lets the latter be tested with a fake client.
func runExport(cmd *cobra.Command, opts *options, listIDs []string, all bool) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()
	errW := cmd.ErrOrStderr()

	if all && len(listIDs) > 0 {
		return coded(exitUsage, errListAndAll)
	}

	if !all && len(listIDs) == 0 {
		return coded(exitUsage, errNoTarget)
	}

	client, err := authenticate(ctx, opts, errW)
	if err != nil {
		return err
	}

	// Convert the raw flag strings into domain IDs at this boundary so the rest
	// of the flow works with typed list IDs.
	ids := make([]export.ListID, len(listIDs))
	for i, id := range listIDs {
		ids[i] = export.ListID(id)
	}

	return runExportWith(ctx, client, opts, ids, all, out, errW)
}

// runExportWith fetches the account's lists, resolves the requested targets, and
// exports each. It takes the client as an interface so it is fully testable.
func runExportWith(ctx context.Context, client taskLister, opts *options, listIDs []export.ListID, all bool, out, errW io.Writer) error {
	lists, err := client.ListTaskLists(ctx)
	if err != nil {
		return coded(exitAPI, friendlyAPIError(err, opts.tokenPath))
	}

	vlogf(errW, opts.verbose, "fetched %d task list(s)\n", len(lists))

	targets, err := resolveTargets(lists, listIDs, all)
	if err != nil {
		return coded(exitUsage, err)
	}

	if len(targets) == 0 {
		fmt.Fprintln(errW, "No task lists to export.")
		return nil
	}

	for _, list := range targets {
		if err := exportList(ctx, client, list, opts, out, errW); err != nil {
			return err
		}
	}

	return nil
}

// resolveTargets maps the requested selection onto concrete task lists. With
// all=true it returns every list; otherwise it returns the lists matching the
// requested IDs, in the order requested, erroring on the first unknown ID.
func resolveTargets(lists []*tasks.TaskList, listIDs []export.ListID, all bool) ([]*tasks.TaskList, error) {
	if all {
		return lists, nil
	}

	byID := make(map[export.ListID]*tasks.TaskList, len(lists))
	for _, l := range lists {
		byID[export.ListID(l.Id)] = l
	}

	targets := make([]*tasks.TaskList, 0, len(listIDs))
	for _, id := range listIDs {
		l, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("%w: %q (run `gtasks export --all` or the interactive picker to see valid IDs)", errListNotFound, id)
		}

		targets = append(targets, l)
	}

	return targets, nil
}
