package cli

import (
	selfupdate "github.com/mrz1836/go-selfupdate"
	"github.com/mrz1836/go-selfupdate/cobracmd"
	"github.com/spf13/cobra"
)

// attachUpdateCommand registers gtask-extractor's self-update command on root
// and wires the passive "a new version is available" notice, both derived from
// a single self-update config. The running binary's compiled-in version is
// threaded in explicitly so a binary run from outside PATH still updates itself.
//
// gtask-extractor installs only from its GitHub release archives — verifying the
// SHA-256 checksum and atomically replacing the binary — and refuses to
// overwrite a binary owned by `go install` or Homebrew. One call registers the
// `update` command (alias `upgrade`, flags --check/--force/--verbose) and the
// banner. AppName derives from BinaryName, giving the GTASK_EXTRACTOR_ env
// prefix (opt out with GTASK_EXTRACTOR_NO_UPDATE_CHECK; the shared
// NO_UPDATE_CHECK and CI also disable it).
func attachUpdateCommand(root *cobra.Command, version string) *cobra.Command {
	cmd := cobracmd.Attach(root, selfupdate.Config{ //nolint:gosec // G101 false positive: TokenEnvVar is an environment variable name, not a credential
		Owner:          "mrz1836",
		Repo:           "gtask-extractor",
		BinaryName:     "gtask-extractor",
		CurrentVersion: version,
		TokenEnvVar:    "GTASK_EXTRACTOR_GITHUB_TOKEN",
	})

	cmd.Example = `  gtask-extractor update
  gtask-extractor update --check
  gtask-extractor update --force`

	return cmd
}
