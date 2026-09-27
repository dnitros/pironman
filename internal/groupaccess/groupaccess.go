// Package groupaccess manages the OS group ADR-0002 uses to gate access to
// the daemon's control socket (see docs/adr/0002-socket-group-permissions.md).
package groupaccess

import (
	"errors"
	"fmt"
	"os/exec"
	"os/user"

	"github.com/dnitros/pironman/internal/ipc"
)

// EnsureGroup creates the pironman group if it doesn't already exist.
var EnsureGroup = func() error {
	var unknownGroupErr user.UnknownGroupError
	if _, err := user.LookupGroup(ipc.GroupName); errors.As(err, &unknownGroupErr) {
		if err := exec.Command("groupadd", ipc.GroupName).Run(); err != nil {
			return fmt.Errorf("create %s group: %w", ipc.GroupName, err)
		}
	} else if err != nil {
		return fmt.Errorf("look up %s group: %w", ipc.GroupName, err)
	}
	return nil
}

// IsMember reports whether username belongs to the pironman group.
var IsMember = func(username string) (bool, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return false, fmt.Errorf("look up user %s: %w", username, err)
	}
	g, err := user.LookupGroup(ipc.GroupName)
	if err != nil {
		return false, fmt.Errorf("look up group %s: %w", ipc.GroupName, err)
	}
	gids, err := u.GroupIds()
	if err != nil {
		return false, fmt.Errorf("look up groups for %s: %w", username, err)
	}
	for _, gid := range gids {
		if gid == g.Gid {
			return true, nil
		}
	}
	return false, nil
}

// AddMember adds username to the pironman group.
var AddMember = func(username string) error {
	if err := exec.Command("usermod", "-aG", ipc.GroupName, username).Run(); err != nil {
		return fmt.Errorf("usermod -aG %s %s: %w", ipc.GroupName, username, err)
	}
	return nil
}
