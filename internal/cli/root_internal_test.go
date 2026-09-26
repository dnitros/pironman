package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandSilencesUsageOnError(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"daemon", "start", "--not-a-real-flag"})

	if err := root.Execute(); err == nil {
		t.Fatalf("expected an error for an unknown flag")
	}
	if strings.Contains(out.String(), "Usage:") {
		t.Fatalf("expected usage to be silenced on error, got: %q", out.String())
	}
}
