package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestAttachUpdateCommand(t *testing.T) {
	root := &cobra.Command{Use: "gtask-extractor"}
	cmd := attachUpdateCommand(root, "1.2.3")

	if cmd.Name() != "update" {
		t.Errorf("self-update command name = %q, want %q", cmd.Name(), "update")
	}

	hasUpgrade := false

	for _, a := range cmd.Aliases {
		if a == "upgrade" {
			hasUpgrade = true
		}
	}

	if !hasUpgrade {
		t.Errorf("update command missing 'upgrade' alias, got %v", cmd.Aliases)
	}

	for _, f := range []string{"check", "force", "verbose"} {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("update command missing --%s flag", f)
		}
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

	if !found {
		t.Error("update command not registered on the root command")
	}
}
