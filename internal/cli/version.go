package cli

import (
	"github.com/spf13/cobra"
)

// newVersionCmd returns the `version` subcommand.
func newVersionCmd(info BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print the gtask-extractor version",
		Example: "  gtask-extractor version",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Printf("gtask-extractor %s\n", formatVersion(info))
			return nil
		},
	}
}
