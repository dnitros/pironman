package selfupdate_test

import (
	"os"
	"strings"
	"testing"

	"github.com/dnitros/pironman/internal/selfupdate"
)

func TestReleaseWorkflowPublishesTheAssetsUpdateExpects(t *testing.T) {
	workflow, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	var publish string
	for _, line := range strings.Split(string(workflow), "\n") {
		if strings.Contains(line, "gh release create") {
			publish = line
		}
	}
	if publish == "" {
		t.Fatalf("release.yml has no gh release create step")
	}
	for _, asset := range []string{selfupdate.BinaryAsset, selfupdate.ChecksumsAsset} {
		if !strings.Contains(publish, " "+asset+" ") {
			t.Fatalf("release.yml publishes %q, missing %q, which pironman update downloads", strings.TrimSpace(publish), asset)
		}
	}
}
