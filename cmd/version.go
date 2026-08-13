package cmd

import (
	"github.com/spf13/cobra"

	"github.com/mrz1836/gtask-extractor/internal/buildinfo"
)

// newVersionCmd returns the `version` subcommand.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the gtasks version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Printf("gtasks %s\n", buildinfo.Version)
			return nil
		},
	}
}
