package groupaccess

import (
	"strings"
	"testing"
)

func TestAddMemberRejectsFlagLikeUsername(t *testing.T) {
	for _, username := range []string{"-p", "--shell=/bin/sh", "; rm -rf /", ""} {
		if err := AddMember(username); err == nil {
			t.Fatalf("expected AddMember(%q) to reject an invalid username, got nil error", username)
		} else if !strings.Contains(err.Error(), "invalid username") {
			t.Fatalf("expected an invalid-username error for %q, got: %v", username, err)
		}
	}
}
