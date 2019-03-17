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
	if err != nil {
		exit(err)
	}
}
