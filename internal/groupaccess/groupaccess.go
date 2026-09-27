package groupaccess

import (
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"regexp"

	"github.com/dnitros/pironman/internal/ipc"
)

var usernamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]*\$?$`)

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

var AddMember = func(username string) error {
	if !usernamePattern.MatchString(username) {
		return fmt.Errorf("invalid username %q", username)
	}
	if err := exec.Command("usermod", "-aG", ipc.GroupName, "--", username).Run(); err != nil {
		return fmt.Errorf("usermod -aG %s %s: %w", ipc.GroupName, username, err)
	}
	return nil
}
