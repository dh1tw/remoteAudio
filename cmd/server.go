package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// serverCmdrepresents the serve command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "remoteAudio Server",
	Long: `Run a remoteAudio server

Start a remoteAudio server using a specific transportation protocol.
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Please select a transportation protocol (--help for available options)")
	},
}

func init() {
	RootCmd.AddCommand(serverCmd)
}
