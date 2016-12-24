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
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Stream Audio through a specfic transportation protocol",
	Long:  `Stream Audio through a specfic transportation protocol`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Please select a transportation protocol (--help for available options)")
	},
}

func init() {
	RootCmd.AddCommand(serveCmd)
	serveCmd.PersistentFlags().StringP("input_device_name", "i", "default", "Input device")
	serveCmd.PersistentFlags().Float64("input_device_sampling_rate", 48000, "Input device sampling rate")
	serveCmd.PersistentFlags().Duration("input_device_latency", time.Millisecond*5, "Input latency")
	serveCmd.PersistentFlags().String("input_device_channels", "mono", "Input Channels")

	serveCmd.PersistentFlags().StringP("output_device_name", "o", "default", "Output device")
	serveCmd.PersistentFlags().Float64("output_device_sampling_rate", 48000, "Output device sampling rate")
	serveCmd.PersistentFlags().Duration("output_device_latency", time.Millisecond*5, "Output latency")
	serveCmd.PersistentFlags().String("output_device_channels", "stereo", "Output Channels")

	serveCmd.PersistentFlags().IntP("buffersize", "b", 1024, "Frames per Buffer")
	serveCmd.PersistentFlags().Float64P("samplingrate", "s", 16000, "sampling rate on the wire")
	serveCmd.PersistentFlags().IntP("bitrate", "B", 16, "Bitrate used on the wire")
	serveCmd.PersistentFlags().String("wire_output_channels", "stereo", "Audio Channels send over the wire")

	viper.BindPFlag("input_device.device_name", serveCmd.PersistentFlags().Lookup("input_device_name"))
	viper.BindPFlag("input_device.samplingrate", serveCmd.PersistentFlags().Lookup("input_device_sampling_rate"))
	viper.BindPFlag("input_device.latency", serveCmd.PersistentFlags().Lookup("input_device_latency"))
	viper.BindPFlag("input_device.channels", serveCmd.PersistentFlags().Lookup("input_device_channels"))

	viper.BindPFlag("output_device.device_name", serveCmd.PersistentFlags().Lookup("output_device_name"))
	viper.BindPFlag("output_device.samplingrate", serveCmd.PersistentFlags().Lookup("output_device_sampling_rate"))
	viper.BindPFlag("output_device.latency", serveCmd.PersistentFlags().Lookup("output_device_latency"))
	viper.BindPFlag("output_device.channels", serveCmd.PersistentFlags().Lookup("output_device_channels"))

	viper.BindPFlag("wire.samplingrate", serveCmd.PersistentFlags().Lookup("samplingrate"))
	viper.BindPFlag("wire.bitrate", serveCmd.PersistentFlags().Lookup("bitrate"))
	viper.BindPFlag("wire.buffersize", serveCmd.PersistentFlags().Lookup("buffersize"))
	viper.BindPFlag("wire.output_channels", serveCmd.PersistentFlags().Lookup("wire_output_channels"))

}
