package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var readBuildInfo = debug.ReadBuildInfo

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show the pironman build version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(versionString())
		},
	}
}

// ponytail: relies solely on go build's auto-embedded VCS metadata, so a
// binary built with -trimpath, -buildvcs=false, or from a release tarball
// without a .git dir always reports "unknown" here. Add an -ldflags -X
// version override at release-build time if that ever needs distinguishing.
func versionString() string {
	info, ok := readBuildInfo()
	if !ok {
		return "unknown"
	}
	return formatVersion(info)
}

func formatVersion(info *debug.BuildInfo) string {
	var revision string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if revision == "" {
		return "unknown"
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if dirty {
		revision += "+dirty"
	}
	return revision
}
