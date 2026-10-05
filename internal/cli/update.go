package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/dnitros/pironman/internal/selfupdate"
	"github.com/dnitros/pironman/internal/systemdunit"
)

type updateEnv struct {
	client    *http.Client
	latestURL string
	binPath   string
	current   string
	mgr       systemdunit.Manager
}

func newUpdateCmd() *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Install the latest pironman release and restart the service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve pironman binary path: %w", err)
			}
			exe, err = filepath.EvalSymlinks(exe)
			if err != nil {
				return fmt.Errorf("resolve pironman binary path: %w", err)
			}
			return runUpdate(updateEnv{
				client:    &http.Client{Timeout: 2 * time.Minute},
				latestURL: selfupdate.LatestURL,
				binPath:   exe,
				current:   versionString(),
				mgr:       systemdunit.NewManager(),
			}, check, os.Stdout)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "report whether a newer release exists without installing it")
	return cmd
}

func runUpdate(env updateEnv, check bool, w io.Writer) error {
	if !check {
		if err := requireRoot("update"); err != nil {
			return err
		}
	}

	rel, err := selfupdate.Latest(env.client, env.latestURL)
	if err != nil {
		return err
	}
	// ponytail: any tag other than the running version counts as an update, so a
	// build newer than the latest release would downgrade; compare semver if that matters.
	if rel.Tag == env.current {
		fmt.Fprintf(w, "already up to date (%s)\n", env.current)
		return nil
	}
	if check {
		fmt.Fprintf(w, "update available: %s → %s\n", env.current, rel.Tag)
		return nil
	}

	bin, err := selfupdate.Download(env.client, rel)
	if err != nil {
		return err
	}
	if err := selfupdate.Replace(env.binPath, bin); err != nil {
		return err
	}

	updated := fmt.Sprintf("updated %s → %s", env.current, rel.Tag)
	if env.mgr.IsSupported() {
		active, err := env.mgr.IsActive()
		if err == nil && active {
			err = env.mgr.Restart()
		}
		if err != nil {
			return fmt.Errorf("%s, but the service was not restarted: %w — run `sudo systemctl restart %s`", updated, err, systemdunit.ServiceName)
		}
	}
	fmt.Fprintln(w, updated)
	return nil
}
