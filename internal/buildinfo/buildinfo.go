// Package buildinfo exposes build-time metadata shared across the program.
package buildinfo

// Version is the gtasks release version. It defaults to "dev" and is
// overridden at build time via -ldflags, e.g.:
//
//	go build -ldflags "-X github.com/mrz1836/gtask-extractor/internal/buildinfo.Version=1.2.3"
//
//nolint:gochecknoglobals // overridden via -ldflags at build time; cannot be a const
var Version = "dev"

// UserAgent returns the HTTP User-Agent string sent to the Google Tasks API.
func UserAgent() string {
	return "gtasks/" + Version
}
