// Copyright © 2017 Tobias Wellnitz, DH1TW <Tobias.Wellnitz@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

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
