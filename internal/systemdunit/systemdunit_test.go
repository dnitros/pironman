package systemdunit_test

import (
	"strings"
	"testing"

	"github.com/dnitros/pironman/internal/systemdunit"
)

func TestUnitContentSetsExecStart(t *testing.T) {
	got := systemdunit.UnitContent("/usr/local/bin/pironman")

	want := "ExecStart=/usr/local/bin/pironman daemon run"
	if !strings.Contains(got, want) {
		t.Fatalf("expected unit content to contain %q, got:\n%s", want, got)
	}
}
