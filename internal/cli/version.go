package cli

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	version       string
	readBuildInfo = debug.ReadBuildInfo
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show the pironman build version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(versionString())
		},
	}
}

func versionString() string {
	if version != "" {
		return version
	}
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
