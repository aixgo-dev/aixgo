package cmd

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of aixgo",
	Long:  `Print the version number of aixgo, along with OS and architecture.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("aixgo version %s %s/%s\n", getVersion(), runtime.GOOS, runtime.GOARCH)
	},
}

// getVersion returns the version string, preferring build info from go install.
func getVersion() string {
	if Version != "dev" && Version != "" {
		return Version
	}

	// Fall back to build info (populated by go install)
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	return Version
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
