package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestAttachUpdateCommand(t *testing.T) {
	root := &cobra.Command{Use: "gtask-extractor"}
	cmd := attachUpdateCommand(root, "1.2.3")

	assert.Equal(t, "update", cmd.Name())

	hasUpgrade := false

	for _, a := range cmd.Aliases {
		if a == "upgrade" {
			hasUpgrade = true
		}
	}

	assert.True(t, hasUpgrade)

	for _, f := range []string{"check", "force", "verbose"} {
		assert.NotNil(t, cmd.Flags().Lookup(f))
	}
}

func TestUpdateCommandWiredIntoRoot(t *testing.T) {
	root := newRootCmd(BuildInfo{Version: "1.2.3"})

	found := false

	for _, c := range root.Commands() {
		if c.Name() == "update" {
			found = true
		}
	}

	assert.True(t, found)
}
