package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// clientCmd represents the connect command
var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "remoteAudio client",
	Long: `Connect to a remoteAudio Server

You have to use the client with a specific transportation protocol.
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Please specify the transportation protocol (--help for available options)")
	},
}

func init() {
	RootCmd.AddCommand(clientCmd)
}
