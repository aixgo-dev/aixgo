package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of aixgo",
	Long:  `Print the version number of aixgo, along with OS and architecture.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("aixgo version %s %s/%s\n", Version, runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
