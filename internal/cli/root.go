// Package cli wires the Cobra command tree: the interactive flow (run when
// gtask-extractor is invoked with no subcommand), the non-interactive `export`
// subcommand, the `version` and self-`update` subcommands, and the shared
// authentication, export, and exit-code-mapping helpers.
package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/api/googleapi"
	tasks "google.golang.org/api/tasks/v1"

	"github.com/mrz1836/gtask-extractor/internal/auth"
	"github.com/mrz1836/gtask-extractor/internal/export"
	"github.com/mrz1836/gtask-extractor/internal/tasksclient"
	"github.com/mrz1836/gtask-extractor/internal/ui"
)

// BuildInfo carries version metadata injected via ldflags (see .goreleaser.yml)
// and threaded in from package main.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// resolvedBuildInfo applies "dev"/"unknown" defaults to empty fields.
func resolvedBuildInfo(info BuildInfo) (version, commit, date string) {
	version, commit, date = info.Version, info.Commit, info.Date
	if version == "" {
		version = "dev"
	}

	if commit == "" {
		commit = "unknown"
	}

	if date == "" {
		date = "unknown"
	}

	return version, commit, date
}

// formatVersion renders a human-friendly version string.
func formatVersion(info BuildInfo) string {
	v, c, d := resolvedBuildInfo(info)
	return fmt.Sprintf("%s (commit: %s, built: %s)", v, c, d)
}

// userAgent is the HTTP User-Agent sent to the Google Tasks API.
func userAgent(version string) string {
	return "gtask-extractor/" + version
}

// Exit codes (documented in the README).
const (
	exitOK      = 0
	exitUsage   = 2   // usage error / not a TTY
	exitAuth    = 3   // credentials or consent problem
	exitAPI     = 4   // API / network failure
	exitOutput  = 5   // writing the export failed
	exitAborted = 130 // interrupted by the user (SIGINT), per convention
)

// Sentinel errors surfaced to the user.
var (
	// errNotInteractive is returned when the interactive flow is run without a terminal.
	errNotInteractive = errors.New(
		"gtask-extractor is interactive and must be run in a terminal (stdin and stdout must be a TTY)",
	)
	// errNoTarget is returned when `export` is invoked without a target.
	errNoTarget = errors.New("nothing to export: pass --list <id> (repeatable) or --all")
	// errListAndAll is returned when `export` mixes --list and --all.
	errListAndAll = errors.New("--list and --all are mutually exclusive")
	// errListNotFound is returned when a requested --list ID is not in the account.
	errListNotFound = errors.New("task list not found")
)

// options holds the resolved global flags (and build version) for a run.
type options struct {
	credsPath string
	tokenPath string
	outputDir string
	verbose   bool
	version   string // resolved build version, for the User-Agent + export metadata
}

// taskLister is the read surface both flows need from the tasks client. It is
// defined here, in the consumer, so tests can drive the flows with a fake.
// It is satisfied by *tasksclient.Client.
type taskLister interface {
	ListTaskLists(ctx context.Context) ([]*tasks.TaskList, error)
	ListTasks(ctx context.Context, listID string) ([]*tasks.Task, error)
}

// newRootCmd builds the command tree. Everything is constructed here (no
// package-level command/flag state and no init()), which keeps the CLI easy to
// test and free of global mutable state.
func newRootCmd(info BuildInfo) *cobra.Command {
	version, _, _ := resolvedBuildInfo(info)
	opts := &options{version: version}

	root := &cobra.Command{
		Use:   "gtask-extractor",
		Short: "Export all of your Google Tasks data to JSON",
		Long: `gtask-extractor exports every field of every task in a chosen Google Tasks
list — including metadata the Tasks UI never shows (updated/created time,
position, completed, hidden, deleted, links and assignment info).

Run it with no arguments for the interactive flow: it authorizes read-only
access, lists your task lists, and exports the one you pick to a JSON file.`,
		Version:       formatVersion(info),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInteractive(cmd, opts)
		},
	}

	pf := root.PersistentFlags()
	pf.StringVar(&opts.credsPath, "creds", "credentials.json", "path to the OAuth client credentials.json")
	pf.StringVar(&opts.tokenPath, "token", "token.json", "path to the cached OAuth token")
	pf.StringVar(&opts.outputDir, "output-dir", "output", "directory for exported JSON files")
	pf.BoolVarP(&opts.verbose, "verbose", "v", false, "enable verbose logging")

	root.SetVersionTemplate("gtask-extractor {{.Version}}\n")
	// Map flag-parse errors to the documented usage exit code (2).
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return coded(exitUsage, err)
	})
	root.AddCommand(newVersionCmd(info), newExportCmd(opts))

	// Register the self-update command (`update`, alias `upgrade`) + passive
	// "new version available" notice, using the running build's version.
	attachUpdateCommand(root, version)

	return root
}

// Execute builds and runs the command tree with the given build info. It prints
// a user-facing error (except on a clean Ctrl-C) and returns the raw error so
// main can map it to a process exit code via ExitCode.
func Execute(info BuildInfo) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := newRootCmd(info).ExecuteContext(ctx)
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		fmt.Fprintln(os.Stderr, "Error:", err)
	}

	return err
}

// ExitCode maps an error returned by Execute to a process exit code.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		// A clean, user-initiated abort (Ctrl-C).
		return exitAborted
	}

	if ce, ok := errors.AsType[*codedError](err); ok {
		return ce.code
	}

	// Anything else is a Cobra usage error (bad flag, wrong args, mutually-
	// exclusive flags); our own RunE always returns a *codedError.
	return exitUsage
}

// runInteractive authenticates once, then loops over list selection + export
// until the user quits.
func runInteractive(cmd *cobra.Command, opts *options) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()  // machine-facing results (the export path)
	errW := cmd.ErrOrStderr() // human-facing prompts and progress

	if !ui.IsInteractive(os.Stdin, os.Stdout) {
		return coded(exitUsage, errNotInteractive)
	}

	vlogf(errW, opts.verbose, "creds=%s token=%s output-dir=%s\n", opts.credsPath, opts.tokenPath, opts.outputDir)

	client, err := authenticate(ctx, opts, errW)
	if err != nil {
		return err
	}

	stdin := bufio.NewReader(os.Stdin) // shared across prompts so buffered input is not lost
	for {
		more, err := exportOnce(ctx, opts, client, stdin, out, errW)
		if err != nil {
			return err
		}

		if !more {
			return nil
		}
	}
}

// exportOnce runs one iteration of the interactive loop: show lists, pick one,
// export it, and ask whether to continue. It returns whether to loop again.
func exportOnce(ctx context.Context, opts *options, client taskLister, stdin io.Reader, out, errW io.Writer) (bool, error) {
	lists, err := client.ListTaskLists(ctx)
	if err != nil {
		return false, coded(exitAPI, friendlyAPIError(err, opts.tokenPath))
	}

	vlogf(errW, opts.verbose, "fetched %d task list(s)\n", len(lists))

	if len(lists) == 0 {
		fmt.Fprintln(errW, "No task lists were found in this account.")
		return false, nil
	}

	fmt.Fprintln(errW, "\nYour task lists:")
	ui.RenderLists(errW, lists)
	fmt.Fprintln(errW)

	idx, quit, err := ui.SelectIndex(errW, stdin,
		fmt.Sprintf("Select a list to export [1-%d] (q to quit): ", len(lists)), len(lists))
	if errors.Is(err, io.EOF) {
		quit = true
	} else if err != nil {
		return false, coded(exitUsage, err)
	}

	if quit {
		fmt.Fprintln(errW, "Goodbye.")
		return false, nil
	}

	if exportErr := exportList(ctx, client, lists[idx], opts, out, errW); exportErr != nil {
		return false, exportErr
	}

	again, err := ui.Confirm(errW, stdin, "\nExport another list?")
	if err != nil {
		return false, coded(exitUsage, err)
	}

	if !again {
		fmt.Fprintln(errW, "Done.")
	}

	return again, nil
}

// exportList exports a single list and prints the result line (stdout) and a
// counts breakdown (stderr). Shared by the interactive and non-interactive flows.
func exportList(ctx context.Context, client export.Lister, list *tasks.TaskList, opts *options, out, errW io.Writer) error {
	fmt.Fprintf(errW, "Exporting %q…\n", list.Title)

	path, counts, err := export.Run(ctx, client, list, export.Options{
		OutDir:      opts.outputDir,
		ToolVersion: opts.version,
		Now:         time.Now,
	})
	if err != nil {
		if isAPIError(err) {
			return coded(exitAPI, friendlyAPIError(err, opts.tokenPath))
		}

		return coded(exitOutput, err)
	}

	fmt.Fprintf(out, "✓ Exported %d tasks from %q → %s\n", counts.Total, list.Title, path)
	fmt.Fprintf(errW, "  active: %d   completed: %d   hidden: %d   deleted: %d   assigned: %d\n",
		counts.NeedsAction, counts.Completed, counts.Hidden, counts.Deleted, counts.Assigned)

	return nil
}

// authenticate loads credentials and returns an authorized tasks client,
// mapping failures to friendly, correctly-coded errors.
func authenticate(ctx context.Context, opts *options, errW io.Writer) (*tasksclient.Client, error) {
	cfg, err := auth.Config(opts.credsPath)
	if err != nil {
		if errors.Is(err, auth.ErrCredentialsNotFound) {
			return nil, coded(exitAuth, fmt.Errorf(
				"%w: no credentials at %s — download an OAuth \"Desktop app\" credentials file from Google Cloud Console and save it there (see the README's setup section)",
				auth.ErrCredentialsNotFound, opts.credsPath,
			))
		}

		return nil, coded(exitAuth, err)
	}

	httpClient, err := auth.Client(ctx, cfg, opts.tokenPath, errW)
	if err != nil {
		return nil, coded(exitAuth, fmt.Errorf(
			"%w\n(if this keeps happening, delete %s and re-run to re-authorize)", err, opts.tokenPath,
		))
	}

	client, err := tasksclient.New(ctx, httpClient, userAgent(opts.version))
	if err != nil {
		return nil, coded(exitAPI, err)
	}

	return client, nil
}

// vlogf writes a diagnostic line to w only when verbose logging is enabled.
func vlogf(w io.Writer, verbose bool, format string, args ...any) {
	if verbose {
		fmt.Fprintf(w, "[verbose] "+format, args...)
	}
}

// isAPIError reports whether err came from the Tasks API.
func isAPIError(err error) bool {
	_, ok := errors.AsType[*googleapi.Error](err)
	return ok
}

// friendlyAPIError adds actionable hints for the common API failures.
func friendlyAPIError(err error, tokenPath string) error {
	if gerr, ok := errors.AsType[*googleapi.Error](err); ok {
		switch gerr.Code {
		case 401:
			return fmt.Errorf("authorization expired or revoked (HTTP 401): delete %s and re-run to re-authorize\n%w", tokenPath, err)
		case 403:
			return fmt.Errorf("access denied (HTTP 403): make sure the Google Tasks API is enabled for your Cloud project and that you granted the read-only Tasks permission\n%w", err)
		}
	}

	return err
}
