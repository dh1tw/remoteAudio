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
	"github.com/dh1tw/remoteAudio/webserver"
	"github.com/gogo/protobuf/proto"
	"github.com/gordonklaus/portaudio"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "net/http/pprof"

	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
)

// connectMqttCmd represents the mqtt command
var connectMqttCmd = &cobra.Command{
	Use:   "mqtt",
	Short: "Client streaming Audio via MQTT",
	Long:  `Client streaming Audio via MQTT`,
	Run: func(cmd *cobra.Command, args []string) {
		mqttAudioClient()
	},
}

func init() {
	connectCmd.AddCommand(connectMqttCmd)
	connectMqttCmd.Flags().StringP("broker-url", "u", "localhost", "Broker URL")
	connectMqttCmd.Flags().StringP("client-id", "c", "", "MQTT Client Id")
	connectMqttCmd.Flags().IntP("broker-port", "p", 1883, "Broker Port")
	connectMqttCmd.Flags().StringP("station", "X", "mystation", "Your station callsign")
	connectMqttCmd.Flags().StringP("radio", "Y", "myradio", "Radio ID")

	// no binding necessary since the flags are already bound in init() of the server
	// bindings are lazy - so they will only be queried during execution!
}

func mqttAudioClient() {

	if viper.GetString("general.user_id") == "" {
		viper.Set("general.user_id", utils.RandStringRunes(10))
	}

	user_id := viper.GetString("general.user_id")

	// defer profile.Start(profile.MemProfile, profile.ProfilePath(".")).Stop()
	// defer profile.Start(profile.CPUProfile, profile.ProfilePath(".")).Stop()
	// defer profile.Start(profile.BlockProfile, profile.ProfilePath(".")).Stop()

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	// viper settings need to be copied in local variables
	// since viper lookups allocate of each lookup a copy
	// and are quite inperformant

	mqttBrokerURL := viper.GetString("mqtt.broker_url")
	mqttBrokerPort := viper.GetInt("mqtt.broker_port")
	mqttClientID := viper.GetString("mqtt.client_id")

	serverBaseTopic := viper.GetString("mqtt.station") +
		"/radios/" + viper.GetString("mqtt.radio") +
		"/audio"

	serverRequestTopic := serverBaseTopic + "/request"
	serverResponseTopic := serverBaseTopic + "/response"

	// errorTopic := baseTopic + "/error"
	serverAudioOutTopic := serverBaseTopic + "/audio_out"
	serverAudioInTopic := serverBaseTopic + "/audio_in"

	mqttTopics := []string{serverResponseTopic, serverAudioOutTopic}

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
	toDeserializeAudioDataCh := make(chan []byte, 20)
	toDeserializeAudioRespCh := make(chan comms.IOMsg, 10)

	evPS := pubsub.New(100)

	var wg sync.WaitGroup

	settings := comms.MqttSettings{
		WaitGroup:  &wg,
		Transport:  "tcp",
		BrokerURL:  mqttBrokerURL,
		BrokerPort: mqttBrokerPort,
		ClientID:   mqttClientID,
		Topics:     mqttTopics,
		ToDeserializeAudioDataCh: toDeserializeAudioDataCh,
		ToDeserializeAudioReqCh:  nil,
		ToDeserializeAudioRespCh: toDeserializeAudioRespCh,
		ToWire:   toWireCh,
		Events:   evPS,
		LastWill: nil,
	}

	webserverSettings := webserver.WebServerSettings{
		Events: evPS,
	}

	player := audio.AudioDevice{
		ToWire:           nil,
		ToSerialize:      nil,
		ToDeserialize:    toDeserializeAudioDataCh,
		AudioToWireTopic: serverAudioOutTopic,
		Events:           evPS,
		WaitGroup:        &wg,
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
		AudioToWireTopic: serverAudioInTopic,
		ToDeserialize:    nil,
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

	wg.Add(3) //mqtt, player, recorder

	go webserver.Webserver(webserverSettings)
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
	reqServerAudioOnCh := evPS.Sub(events.RequestServerAudioOn)
	osExitCh := evPS.Sub(events.OsExit)
	shutdownCh := evPS.Sub(events.Shutdown)

	pingTicker := time.NewTicker(time.Second)

	// local states
	connectionStatus := comms.DISCONNECTED
	serverOnline := false
	serverAudioOn := false

	for {
		select {

		// pre shutdown hooks (CTRL-C)
		case <-osExitCh:
			evPS.Pub(true, events.Shutdown)

		// shutdown the application gracefully
		case <-shutdownCh:
			wg.Wait()
			os.Exit(0)

		// connection has been established
		case ev := <-connectionStatusCh:
			connectionStatus = ev.(int)
			// evPS.Pub(connectionStatus, events.MqttConnStatus)

		// send ping if connected to Broker
		case <-pingTicker.C:
			if connectionStatus == comms.CONNECTED {
				sendPing(user_id, serverRequestTopic, toWireCh)
			}

		case ev := <-reqServerAudioOnCh:
			if connectionStatus == comms.CONNECTED {
				if err := sendClientRequest(ev.(bool), serverRequestTopic, toWireCh); err != nil {
					fmt.Println(err)
				}
			}

		// responses coming from server
		case data := <-toDeserializeAudioRespCh:

			msg := sbAudio.ServerResponse{}

			err := proto.Unmarshal(data.Data, &msg)
			if err != nil {
				fmt.Println(err)
			}

			if msg.Online != nil {
				serverOnline = msg.GetOnline()
				// fmt.Println("Server Online:", serverOnline)
				evPS.Pub(serverOnline, events.ServerOnline)
			}

			if msg.AudioStream != nil {
				serverAudioOn = msg.GetAudioStream()
				// fmt.Printf("Server Audio is %t\n", serverAudioOn)
				evPS.Pub(serverAudioOn, events.ServerAudioOn)
			}

			if msg.TxUser != nil {
				txUser := msg.GetTxUser()
				// fmt.Printf("Server: Current TX User: %s\n", txUser)
				evPS.Pub(txUser, events.TxUser)
			}

			if msg.PingOrigin != nil && msg.Pong != nil {
				if msg.GetPingOrigin() == user_id {
					pong := time.Unix(0, msg.GetPong())
					delta := time.Since(pong)
					// fmt.Println("Ping:", delta.Nanoseconds()/1000000, "ms")
					evPS.Pub(delta.Nanoseconds(), events.Ping)
				}
			}
		}
	}
}

func sendClientRequest(audioStreamOn bool, topic string, toWireCh chan comms.IOMsg) error {
	req := sbAudio.ClientRequest{}
	req.AudioStream = &audioStreamOn
	m, err := req.Marshal()
	if err != nil {
		fmt.Println(err)
	} else {
		wireMsg := comms.IOMsg{
			Topic: topic,
			Data:  m,
		}
		toWireCh <- wireMsg
	}

	return nil
}

func sendPing(user_id, topic string, toWireCh chan comms.IOMsg) {
	now := time.Now().UnixNano()

	req := sbAudio.ClientRequest{}
	req.PingOrigin = &user_id
	req.Ping = &now

	data, err := req.Marshal()
	if err != nil {
		fmt.Println(err)
	} else {
		wireMsg := comms.IOMsg{
			Topic: topic,
			Data:  data,
		}
		toWireCh <- wireMsg
	}
}
