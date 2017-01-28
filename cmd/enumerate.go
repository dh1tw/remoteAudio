// Copyright © 2016 Tobias Wellnitz, DH1TW <Tobias.Wellnitz@gmail.com>
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
	"os"
	"text/template"

	"github.com/gordonklaus/portaudio"
	"github.com/spf13/cobra"
)

// enumerateCmd represents the enumerate command
var enumerateCmd = &cobra.Command{
	Use:   "enumerate",
	Short: "List all available audio devices and supported Host APIs",
	Long:  `List all available audio devices and supported Host APIs`,
	Run: func(cmd *cobra.Command, args []string) {
		enumerate()
	},
}

func init() {
	RootCmd.AddCommand(enumerateCmd)
}

var tmpl = template.Must(template.New("").Parse(
	`
Available audio devices and supported Host APIs:

	Detected {{. | len}} host API(s): {{range .}}
	
	Name:                   {{.Name}}
	{{if .DefaultInputDevice}}Default input device:   {{.DefaultInputDevice.Name}}{{end}}
	{{if .DefaultOutputDevice}}Default output device:  {{.DefaultOutputDevice.Name}}{{end}}
	Devices: {{range .Devices}}
		Name:                      {{.Name}}
		MaxInputChannels:          {{.MaxInputChannels}}
		MaxOutputChannels:         {{.MaxOutputChannels}}
		DefaultLowInputLatency:    {{.DefaultLowInputLatency}}
		DefaultLowOutputLatency:   {{.DefaultLowOutputLatency}}
		DefaultHighInputLatency:   {{.DefaultHighInputLatency}}
		DefaultHighOutputLatency:  {{.DefaultHighOutputLatency}}
		DefaultSampleRate:         {{.DefaultSampleRate}}
	{{end}}
{{end}}`,
))

// enumerate lists all available Audio devices on the system
func enumerate() {
	portaudio.Initialize()
	defer portaudio.Terminate()

	hs, err := portaudio.HostApis()
	if err != nil {
		fmt.Println(err)
	}
	err = tmpl.Execute(os.Stdout, hs)
}
