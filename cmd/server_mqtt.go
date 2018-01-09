package cmd

import (
	"fmt"
	"os"
	"strings"
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

	// _ "net/http/pprof"

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
	serverMqttCmd.Flags().StringP("broker-url", "u", "test.mosquitto.org", "MQTT Broker URL")
	serverMqttCmd.Flags().IntP("broker-port", "p", 1883, "MQTT Broker Port")
	serverMqttCmd.Flags().StringP("username", "U", "", "MQTT Username")
	serverMqttCmd.Flags().StringP("password", "P", "", "MQTT Password")
	serverMqttCmd.Flags().StringP("client-id", "C", "remoteAudio-svr", "MQTT ClientID")
	serverMqttCmd.Flags().StringP("station", "X", "mystation", "Your station callsign")
	serverMqttCmd.Flags().StringP("radio", "Y", "myradio", "Radio ID")
}

func mqttAudioServer(cmd *cobra.Command, args []string) {

	// Try to read config file
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		if strings.Contains(err.Error(), "Not Found in") {
			fmt.Println("no config file found")
		} else {
			fmt.Println("Error parsing config file", viper.ConfigFileUsed())
			fmt.Println(err)
			os.Exit(-1)
		}
	}

	// check if values from config file / pflags are valid
	if !checkAudioParameterValues() {
		os.Exit(-1)
	}

	// bind the pflags to viper settings
	viper.BindPFlag("mqtt.username", cmd.Flags().Lookup("username"))
	viper.BindPFlag("mqtt.password", cmd.Flags().Lookup("password"))
	viper.BindPFlag("mqtt.client-id", cmd.Flags().Lookup("client-id"))
	viper.BindPFlag("mqtt.broker-url", cmd.Flags().Lookup("broker-url"))
	viper.BindPFlag("mqtt.broker-port", cmd.Flags().Lookup("broker-port"))
	viper.BindPFlag("mqtt.station", cmd.Flags().Lookup("station"))
	viper.BindPFlag("mqtt.radio", cmd.Flags().Lookup("radio"))

	// profiling server
	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	// viper settings need to be copied in local variables
	// since viper lookups allocate of each lookup a copy
	// and are quite inperformant

	mqttBrokerURL := viper.GetString("mqtt.broker-url")
	mqttBrokerPort := viper.GetInt("mqtt.broker-port")
	mqttClientID := viper.GetString("mqtt.client-id")
	mqttUsername := viper.GetString("mqtt.username")
	mqttPassword := viper.GetString("mqtt.password")

	if mqttClientID == "remoteAudio-svr" {
		mqttClientID = mqttClientID + "-" + utils.RandStringRunes(5)
		// update the viper key since it will be retrieved in other parts
		// of the application
		viper.Set("mqtt.client-id", mqttClientID)
	}

	baseTopic := viper.GetString("mqtt.station") +
		"/radios/" + viper.GetString("mqtt.radio") +
		"/audio"

	serverRequestTopic := baseTopic + "/request"
	serverResponseTopic := baseTopic + "/response"
	serverAudioOutTopic := baseTopic + "/audio_out"
	serverAudioInTopic := baseTopic + "/audio_in"

	// errorTopic := baseTopic + "/error"

	mqttTopics := []string{serverRequestTopic, serverAudioInTopic}

	audioFrameLength := viper.GetInt("audio.frame-length")

	outputDeviceDeviceName := viper.GetString("output-device.device-name")
	outputDeviceSamplingrate := viper.GetFloat64("output-device.samplingrate")
	outputDeviceLatency := viper.GetDuration("output-device.latency")
	outputDeviceChannels := viper.GetString("output-device.channels")

	inputDeviceDeviceName := viper.GetString("input-device.device-name")
	inputDeviceSamplingrate := viper.GetFloat64("input-device.samplingrate")
	inputDeviceLatency := viper.GetDuration("input-device.latency")
	inputDeviceChannels := viper.GetString("input-device.channels")

	portaudio.Initialize()
	defer portaudio.Terminate()

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
		Username:   mqttUsername,
		Password:   mqttPassword,
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
