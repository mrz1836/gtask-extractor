// Command gtasks exports all of your Google Tasks data for a chosen list to
// JSON, capturing every field including metadata the Tasks UI never shows.
package main

import (
	"os"

	"github.com/mrz1836/gtask-extractor/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
