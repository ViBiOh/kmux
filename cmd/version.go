package cmd

import (
	"github.com/ViBiOh/kmux/pkg/output"
	"github.com/spf13/cobra"
)

var Version string

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		output.Std("", "%s", Version)
	},
}
