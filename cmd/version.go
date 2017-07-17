package cmd

import (
	"fmt"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

var version string
var commitHash string

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of remoteAudio",
	Long:  `All software has versions. This is remoteAudio's.`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Work your own magic here
		printRemoteAudioVersion()
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}

func printRemoteAudioVersion() {
	buildDate := time.Now().Format(time.RFC3339)
	fmt.Printf("remoteAudio Version: %s, %s/%s, BuildDate: %s, Commit: %s\n",
		version, runtime.GOOS, runtime.GOARCH, buildDate, commitHash)
}
