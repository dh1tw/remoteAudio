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
	"os"
	"sync"
	"time"

	"github.com/cskr/pubsub"
	"github.com/dh1tw/remoteAudio/audio"
	"github.com/dh1tw/remoteAudio/comms"
	"github.com/dh1tw/remoteAudio/events"
	"github.com/dh1tw/remoteAudio/utils"
	"github.com/gogo/protobuf/proto"
	"github.com/gordonklaus/portaudio"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "net/http/pprof"

	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
)

// serverMqttCmd represents the mqtt command
var serverMqttCmd = &cobra.Command{
	Use:   "mqtt",
	Short: "MQTT Server for bi-directional audio streaming",
	Long:  `MQTT Server for bi-directional audio streaming`,
	Run:   mqttAudioServer,
}

func init() {
	serverCmd.AddCommand(serverMqttCmd)
	serverMqttCmd.Flags().StringP("broker-url", "u", "localhost", "Broker URL")
	serverMqttCmd.Flags().IntP("broker-port", "p", 1883, "Broker Port")
	serverMqttCmd.Flags().StringP("station", "X", "mystation", "Your station callsign")
	serverMqttCmd.Flags().StringP("radio", "Y", "myradio", "Radio ID")
}

func mqttAudioServer(cmd *cobra.Command, args []string) {

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}

	// bind the pflags to viper settings
	viper.BindPFlag("mqtt.broker_url", cmd.Flags().Lookup("broker-url"))
	viper.BindPFlag("mqtt.broker_port", cmd.Flags().Lookup("broker-port"))
	viper.BindPFlag("mqtt.station", cmd.Flags().Lookup("station"))
	viper.BindPFlag("mqtt.radio", cmd.Flags().Lookup("radio"))

	if viper.GetString("general.user_id") == "" {
		viper.Set("general.user_id", utils.RandStringRunes(10))
	}

	// profiling server can be enabled through a hidden pflag
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	// viper settings need to be copied in local variables
	// since viper lookups allocate of each lookup a copy
	// and are quite inperformant

	mqttBrokerURL := viper.GetString("mqtt.broker_url")
	mqttBrokerPort := viper.GetInt("mqtt.broker_port")
	mqttClientID := viper.GetString("general.user_id")

	baseTopic := viper.GetString("mqtt.station") +
		"/radios/" + viper.GetString("mqtt.radio") +
		"/audio"

	serverRequestTopic := baseTopic + "/request"
	serverResponseTopic := baseTopic + "/response"
	serverAudioOutTopic := baseTopic + "/audio_out"
	serverAudioInTopic := baseTopic + "/audio_in"

	// errorTopic := baseTopic + "/error"

	mqttTopics := []string{serverRequestTopic, serverAudioInTopic}

	audioFrameLength := viper.GetInt("audio.frame_length")

	outputDeviceDeviceName := viper.GetString("output_device.device_name")
	outputDeviceSamplingrate := viper.GetFloat64("output_device.samplingrate")
	outputDeviceLatency := viper.GetDuration("output_device.latency")
	outputDeviceChannels := viper.GetString("output_device.channels")

	inputDeviceDeviceName := viper.GetString("input_device.device_name")
	inputDeviceSamplingrate := viper.GetFloat64("input_device.samplingrate")
	inputDeviceLatency := viper.GetDuration("input_device.latency")
	inputDeviceChannels := viper.GetString("input_device.channels")

	portaudio.Initialize()

	toWireCh := make(chan comms.IOMsg, 20)
	toSerializeAudioDataCh := make(chan comms.IOMsg, 20)
	toDeserializeAudioDataCh := make(chan []byte, 10)
	toDeserializeAudioReqCh := make(chan comms.IOMsg, 10)

	// Event PubSub
	evPS := pubsub.New(1)

	// WaitGroup to coordinate a graceful shutdown
	var wg sync.WaitGroup

	// mqtt Last Will Message
	binaryWillMsg, err := createLastWillMsg()
	if err != nil {
		fmt.Println(err)
	}

	lastWill := comms.LastWill{
		Topic:  serverResponseTopic,
		Data:   binaryWillMsg,
		Qos:    0,
		Retain: true,
	}

	settings := comms.MqttSettings{
		WaitGroup:  &wg,
		Transport:  "tcp",
		BrokerURL:  mqttBrokerURL,
		BrokerPort: mqttBrokerPort,
		ClientID:   mqttClientID,
		Topics:     mqttTopics,
		ToDeserializeAudioDataCh: toDeserializeAudioDataCh,
		ToDeserializeAudioReqCh:  toDeserializeAudioReqCh,
		ToWire:                   toWireCh,
		Events:                   evPS,
		LastWill:                 &lastWill,
	}

	player := audio.AudioDevice{
		ToWire:        nil,
		ToSerialize:   nil,
		ToDeserialize: toDeserializeAudioDataCh,
		Events:        evPS,
		WaitGroup:     &wg,
		AudioStream: audio.AudioStream{
			DeviceName:      outputDeviceDeviceName,
			FramesPerBuffer: audioFrameLength,
			Samplingrate:    outputDeviceSamplingrate,
			Latency:         outputDeviceLatency,
			Channels:        audio.GetChannel(outputDeviceChannels),
		},
	}

	recorder := audio.AudioDevice{
		ToWire:           toWireCh,
		ToSerialize:      toSerializeAudioDataCh,
		ToDeserialize:    nil,
		AudioToWireTopic: serverAudioOutTopic,
		Events:           evPS,
		WaitGroup:        &wg,
		AudioStream: audio.AudioStream{
			DeviceName:      inputDeviceDeviceName,
			FramesPerBuffer: audioFrameLength,
			Samplingrate:    inputDeviceSamplingrate,
			Latency:         inputDeviceLatency,
			Channels:        audio.GetChannel(inputDeviceChannels),
		},
	}

	wg.Add(3) //recorder, player and MQTT

	go events.WatchSystemEvents(evPS)
	go audio.PlayerASync(player)
	// give the Audio Streams time to setup and start
	time.Sleep(time.Millisecond * 150)
	go audio.RecorderAsync(recorder)
	// give the Audio Streams time to setup and start
	time.Sleep(time.Millisecond * 150)
	go comms.MqttClient(settings)
	// go events.CaptureKeyboard(evPS)

	connectionStatusCh := evPS.Sub(events.MqttConnStatus)
	txUserCh := evPS.Sub(events.TxUser)
	recordAudioOnCh := evPS.Sub(events.RecordAudioOn)
	osExitCh := evPS.Sub(events.OsExit)
	shutdownCh := evPS.Sub(events.Shutdown)

	status := serverStatus{}
	status.topic = serverResponseTopic

	for {
		select {

		// CTRL-C has been pressed; let's prepare the shutdown
		case <-osExitCh:
			// advice that we are going offline
			status.online = false
			status.recordAudioOn = false
			if err := status.sendUpdate(toWireCh); err != nil {
				fmt.Println(err)
			}
			time.Sleep(time.Millisecond * 200)
			evPS.Pub(true, events.Shutdown)

		// shutdown the application gracefully
		case <-shutdownCh:
			wg.Wait()
			os.Exit(0)

		case ev := <-connectionStatusCh:
			connStatus := ev.(int)
			if connStatus == comms.CONNECTED {
				status.online = true
				if err := status.sendUpdate(toWireCh); err != nil {
					fmt.Println(err)
				}
			} else {
				status.online = false
			}

		case ev := <-recordAudioOnCh:
			status.recordAudioOn = ev.(bool)
			if err := status.sendUpdate(toWireCh); err != nil {
				fmt.Println(err)
			}

		case ev := <-txUserCh:
			status.txUser = ev.(string)
			if err := status.sendUpdate(toWireCh); err != nil {
				fmt.Println(err)
			}

		case data := <-toDeserializeAudioReqCh:

			msg := sbAudio.ClientRequest{}

			err := proto.Unmarshal(data.Data, &msg)
			if err != nil {
				fmt.Println(err)
			}

			if msg.AudioStream != nil {
				status.recordAudioOn = msg.GetAudioStream()
				evPS.Pub(status.recordAudioOn, events.RecordAudioOn)
			}

			if msg.Ping != nil && msg.PingOrigin != nil {
				status.pingOrigin = msg.GetPingOrigin()
				status.pong = msg.GetPing()
			}

			if err := status.sendUpdate(toWireCh); err != nil {
				fmt.Println(err)
			}
		}
	}
}

type serverStatus struct {
	online        bool
	recordAudioOn bool
	txUser        string
	topic         string
	pingOrigin    string
	pong          int64
}

func (status *serverStatus) clearPing() {
	status.pingOrigin = ""
	status.pong = -1
}

func (status *serverStatus) sendUpdate(toWireCh chan comms.IOMsg) error {

	now := time.Now().Unix()
	defer status.clearPing()

	msg := sbAudio.ServerResponse{}
	msg.LastSeen = &now
	msg.Online = &status.online
	msg.AudioStream = &status.recordAudioOn
	msg.TxUser = &status.txUser
	msg.PingOrigin = &status.pingOrigin
	msg.Pong = &status.pong

	data, err := msg.Marshal()
	if err != nil {
		return err
	}

	m := comms.IOMsg{}
	m.Data = data
	m.Topic = status.topic
	m.Retain = true

	toWireCh <- m

	return nil

}

func createLastWillMsg() ([]byte, error) {

	willMsg := sbAudio.ServerResponse{}
	online := false
	audioOn := false
	willMsg.Online = &online
	willMsg.AudioStream = &audioOn
	data, err := willMsg.Marshal()

	return data, err
}
