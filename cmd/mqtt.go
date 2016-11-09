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

	"github.com/cskr/pubsub"
	"github.com/dh1tw/remoteAudio/audio"
	"github.com/dh1tw/remoteAudio/comms"
	"github.com/dh1tw/remoteAudio/events"
	"github.com/gordonklaus/portaudio"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// mqttCmd represents the mqtt command
var mqttCmd = &cobra.Command{
	Use:   "mqtt",
	Short: "Stream Audio via MQTT",
	Long:  `Stream Audio via MQTT`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Work your own magic here
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
	portaudio.Initialize()

	connStatus := pubsub.New(1)

	audioInCh := make(chan audio.AudioMsg)
	audioOutCh := make(chan audio.AudioMsg)
	audioLoopbackCh := make(chan audio.AudioMsg)

	evPS := pubsub.New(1)

	settings := comms.MqttSettings{
		Transport:  "tcp",
		BrokerURL:  viper.GetString("mqtt.broker_url"),
		BrokerPort: viper.GetInt("mqtt.broker_port"),
		ClientID:   viper.GetString("mqtt.client_id"),
		Topics:     []string{viper.GetString("mqtt.topic_audio_in")},
		AudioInCh:  audioInCh,
		AudioOutCh: audioOutCh,
		ConnStatus: *connStatus,
	}

	player := audio.AudioDevice{
		AudioInCh:       audioInCh,
		AudioOutCh:      nil,
		AudioLoopbackCh: audioLoopbackCh,
		EventCh:         evPS.Sub(events.EVENTS),
		AudioStream: audio.AudioStream{
			DeviceName:      viper.GetString("audio.output_device"),
			FramesPerBuffer: viper.GetInt("audio.buffersize"),
			Samplingrate:    viper.GetFloat64("audio.samplingrate"),
			Latency:         viper.GetDuration("audio.output_latency"),
			Channels:        audio.MONO,
		},
	}

	recorder := audio.AudioDevice{
		AudioInCh:       nil,
		AudioOutCh:      audioOutCh,
		AudioLoopbackCh: audioLoopbackCh,
		EventCh:         evPS.Sub(events.EVENTS),
		AudioStream: audio.AudioStream{
			DeviceName:      viper.GetString("audio.output_device"),
			FramesPerBuffer: viper.GetInt("audio.buffersize"),
			Samplingrate:    viper.GetFloat64("audio.samplingrate"),
			Latency:         viper.GetDuration("audio.output_latency"),
			Channels:        audio.MONO,
		},
	}

	go audio.PlayerSync(player)
	go audio.RecorderSync(recorder)

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
