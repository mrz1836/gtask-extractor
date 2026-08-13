// Package main is the entry point for the gtask-extractor CLI.
package main

import (
	"os"

	"github.com/mrz1836/gtask-extractor/internal/cli"
)

// Build info variables set via ldflags during build (see .goreleaser.yml).
//
//nolint:gochecknoglobals // required for ldflags injection at build time
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	os.Exit(cli.ExitCode(cli.Execute(cli.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    buildDate,
	})))
}
