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
	"log"
	"net/http"

	"github.com/cskr/pubsub"
	"github.com/dh1tw/remoteAudio/audio"
	"github.com/dh1tw/remoteAudio/comms"
	"github.com/dh1tw/remoteAudio/events"
	"github.com/gordonklaus/portaudio"
	"github.com/pkg/profile"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "net/http/pprof"
)

// mqttCmd represents the mqtt command
var mqttCmd = &cobra.Command{
	Use:   "mqtt",
	Short: "Stream Audio via MQTT",
	Long:  `Stream Audio via MQTT`,
	Run: func(cmd *cobra.Command, args []string) {
		audioClient()
	},
}

func init() {
	serveCmd.AddCommand(mqttCmd)
	mqttCmd.PersistentFlags().StringP("broker_url", "u", "localhost", "Broker URL")
	mqttCmd.PersistentFlags().StringP("client_id", "c", "", "MQTT Client Id")
	mqttCmd.PersistentFlags().IntP("broker_port", "p", 1883, "Broker Port")
	mqttCmd.PersistentFlags().StringP("topic_audio_out", "O", "station/radios/myradio/audio/out", "Topic where outgoing audio is published")
	mqttCmd.PersistentFlags().StringP("topic_audio_in", "I", "station/radios/myradio/audio/in", "Topic for incoming audio")
	viper.BindPFlag("mqtt.broker_url", mqttCmd.PersistentFlags().Lookup("broker_url"))
	viper.BindPFlag("mqtt.broker_port", mqttCmd.PersistentFlags().Lookup("broker_port"))
	viper.BindPFlag("mqtt.client_id", mqttCmd.PersistentFlags().Lookup("client_id"))
	viper.BindPFlag("mqtt.topic_audio_out", mqttCmd.PersistentFlags().Lookup("topic_audio_out"))
	viper.BindPFlag("mqtt.topic_audio_in", mqttCmd.PersistentFlags().Lookup("topic_audio_in"))
}

func audioClient() {

	// defer profile.Start(profile.MemProfile, profile.ProfilePath(".")).Stop()
	defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()
	defer profile.Start(profile.BlockProfile, profile.ProfilePath(".")).Stop()

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	portaudio.Initialize()

	connStatus := pubsub.New(1)

	toWireCh := make(chan audio.AudioMsg, 5)
	toSerializeCh := make(chan audio.AudioMsg, 5)
	toDeserializeCh := make(chan audio.AudioMsg, 10)
	audioLoopbackCh := make(chan audio.AudioMsg)

	evPS := pubsub.New(1)

	// viper settings need to be copied in local variables
	// since viper lookups allocate of each lookup a copy
	// and are quite inperformant

	mqttBrokerURL := viper.GetString("mqtt.broker_url")
	mqttBrokerPort := viper.GetInt("mqtt.broker_port")
	mqttClientID := viper.GetString("mqtt.client_id")
	mqttTopics := []string{viper.GetString("mqtt.topic_audio_in")}

	wireBuffersize := viper.GetInt("wire.buffersize")

	outputDeviceDeviceName := viper.GetString("output_device.device_name")
	outputDeviceSamplingrate := viper.GetFloat64("output_device.samplingrate")
	outputDeviceLatency := viper.GetDuration("output_device.latency")
	outputDeviceChannels := viper.GetString("output_device.channels")

	inputDeviceDeviceName := viper.GetString("input_device.device_name")
	inputDeviceSamplingrate := viper.GetFloat64("input_device.samplingrate")
	inputDeviceLatency := viper.GetDuration("input_device.latency")
	inputDeviceChannels := viper.GetString("input_device.channels")

	settings := comms.MqttSettings{
		Transport:  "tcp",
		BrokerURL:  mqttBrokerURL,
		BrokerPort: mqttBrokerPort,
		ClientID:   mqttClientID,
		Topics:     mqttTopics,
		FromWire:   toDeserializeCh,
		ToWire:     toWireCh,
		ConnStatus: *connStatus,
	}

	player := audio.AudioDevice{
		ToWire:          nil,
		ToSerialize:     nil,
		ToDeserialize:   toDeserializeCh,
		AudioLoopbackCh: audioLoopbackCh,
		EventCh:         evPS.Sub(events.EVENTS),
		AudioStream: audio.AudioStream{
			DeviceName:      outputDeviceDeviceName,
			FramesPerBuffer: wireBuffersize,
			Samplingrate:    outputDeviceSamplingrate,
			Latency:         outputDeviceLatency,
			Channels:        audio.GetChannel(outputDeviceChannels),
		},
	}

	recorder := audio.AudioDevice{
		ToWire:          toWireCh,
		ToSerialize:     toSerializeCh,
		ToDeserialize:   nil,
		AudioLoopbackCh: audioLoopbackCh,
		EventCh:         evPS.Sub(events.EVENTS),
		AudioStream: audio.AudioStream{
			DeviceName:      inputDeviceDeviceName,
			FramesPerBuffer: wireBuffersize,
			Samplingrate:    inputDeviceSamplingrate,
			Latency:         inputDeviceLatency,
			Channels:        audio.GetChannel(inputDeviceChannels),
		},
	}

	go audio.PlayerSync(player)
	go audio.RecorderAsync(recorder)

	go comms.MqttClient(settings)

	eventsConf := events.EventsConf{
		EventsPubSub: evPS,
	}

	go events.CaptureKeyboard(eventsConf)

	connectionStatusCh := connStatus.Sub(comms.CONNSTATUSTOPIC)
	eventsCh := evPS.Sub(events.EVENTS)

	for {
		select {
		case status := <-connectionStatusCh:
			fmt.Println(status)
		case event := <-eventsCh:
			fmt.Println(event)
		}
	}
}
