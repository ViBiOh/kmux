package cmd

import (
	"runtime/debug"

	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/spf13/cobra"
)

// Version is set at release time, it falls back to the module version when
// installed with `go install`.
var Version string

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(_ *cobra.Command, _ []string) {
		output.Std("", "%s", version())
	},
}

func version() string {
	if len(Version) != 0 {
		return Version
	}

	if info, ok := debug.ReadBuildInfo(); ok && len(info.Main.Version) != 0 {
		return info.Main.Version
	}

	return "(devel)"
}
