package cmd

// import (
// 	"fmt"
// 	"os"
// 	"strings"
// 	"sync"
// 	"time"

// 	"github.com/cskr/pubsub"
// 	"github.com/dh1tw/remoteAudio/audio"
// 	"github.com/dh1tw/remoteAudio/comms"
// 	"github.com/dh1tw/remoteAudio/events"
// 	"github.com/dh1tw/remoteAudio/utils"
// 	"github.com/dh1tw/remoteAudio/webserver"
// 	"github.com/gogo/protobuf/proto"
// 	"github.com/gordonklaus/portaudio"
// 	"github.com/spf13/cobra"
// 	"github.com/spf13/viper"

// 	// _ "net/http/pprof"

// 	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
// )

// // clientMqttCmd represents the mqtt command
// var clientMqttCmd = &cobra.Command{
// 	Use:   "mqtt",
// 	Short: "MQTT Client for bi-directional audio streaming",
// 	Long: `MQTT Client for bi-directional audio streaming

// This server implements the Shackbus.org specification (https://shackbus.org).
// 	`,
// 	Run: mqttAudioClient,
// }

// func init() {
// 	clientCmd.AddCommand(clientMqttCmd)
// 	clientMqttCmd.Flags().StringP("broker-url", "u", "test.mosquitto.org", "MQTT Broker URL")
// 	clientMqttCmd.Flags().IntP("broker-port", "p", 1883, "MQTT Broker Port")
// 	clientMqttCmd.Flags().StringP("username", "U", "", "MQTT Username")
// 	clientMqttCmd.Flags().StringP("password", "P", "", "MQTT Password")
// 	clientMqttCmd.Flags().StringP("client-id", "C", "remoteAudio-client", "MQTT ClientID")
// 	clientMqttCmd.Flags().StringP("station", "X", "mystation", "Station you want to connect to")
// 	clientMqttCmd.Flags().StringP("radio", "Y", "myradio", "ID of radio you want to connect to")
// 	clientMqttCmd.Flags().Bool("webui-disabled", false, "WebUI server disabled")
// 	clientMqttCmd.Flags().String("webui-address", "127.0.0.1", "WebUI - Address of Server")
// 	clientMqttCmd.Flags().Int("webui-port", 8080, "WebUI - Port of Server")
// }

// func mqttAudioClient(cmd *cobra.Command, args []string) {

// 	// Try to read config file
// 	if err := viper.ReadInConfig(); err == nil {
// 		fmt.Println("Using config file:", viper.ConfigFileUsed())
// 	} else {
// 		if strings.Contains(err.Error(), "Not Found in") {
// 			fmt.Println("no config file found")
// 		} else {
// 			fmt.Println("Error parsing config file", viper.ConfigFileUsed())
// 			fmt.Println(err)
// 			os.Exit(-1)
// 		}
// 	}

// 	// check if values from config file / pflags are valid
// 	if !checkAudioParameterValues() {
// 		os.Exit(-1)
// 	}

// 	// bind the pflags to viper settings
// 	viper.BindPFlag("mqtt.username", cmd.Flags().Lookup("username"))
// 	viper.BindPFlag("mqtt.password", cmd.Flags().Lookup("password"))
// 	viper.BindPFlag("mqtt.client-id", cmd.Flags().Lookup("client-id"))
// 	viper.BindPFlag("mqtt.broker-url", cmd.Flags().Lookup("broker-url"))
// 	viper.BindPFlag("mqtt.broker-port", cmd.Flags().Lookup("broker-port"))
// 	viper.BindPFlag("mqtt.station", cmd.Flags().Lookup("station"))
// 	viper.BindPFlag("mqtt.radio", cmd.Flags().Lookup("radio"))

// 	viper.BindPFlag("webui.disabled", cmd.Flags().Lookup("webui-disabled"))
// 	viper.BindPFlag("webui.address", cmd.Flags().Lookup("webui-address"))
// 	viper.BindPFlag("webui.port", cmd.Flags().Lookup("webui-port"))

// 	mqttBrokerURL := viper.GetString("mqtt.broker-url")
// 	mqttBrokerPort := viper.GetInt("mqtt.broker-port")
// 	mqttClientID := viper.GetString("mqtt.client-id")
// 	mqttUsername := viper.GetString("mqtt.username")
// 	mqttPassword := viper.GetString("mqtt.password")

// 	if mqttClientID == "remoteAudio-client" {
// 		mqttClientID = mqttClientID + "-" + utils.RandStringRunes(5)
// 		// update the viper key since it will be retrieved in other parts
// 		// of the application
// 		viper.Set("mqtt.client-id", mqttClientID)
// 	}
// 	// profiling server
// 	// go func() {
// 	// 	log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
// 	// }()

// 	// viper settings need to be copied in local variables
// 	// since viper lookups allocate of each lookup a copy
// 	// and are quite inperformant

// 	serverBaseTopic := viper.GetString("mqtt.station") +
// 		"/radios/" + viper.GetString("mqtt.radio") +
// 		"/audio"

// 	serverRequestTopic := serverBaseTopic + "/request"
// 	serverResponseTopic := serverBaseTopic + "/response"

// 	// errorTopic := baseTopic + "/error"
// 	serverAudioOutTopic := serverBaseTopic + "/audio_out"
// 	serverAudioInTopic := serverBaseTopic + "/audio_in"

// 	mqttTopics := []string{serverResponseTopic, serverAudioOutTopic}

// 	audioFrameLength := viper.GetInt("audio.frame-length")

// 	outputDeviceDeviceName := viper.GetString("output-device.device-name")
// 	outputDeviceSamplingrate := viper.GetFloat64("output-device.samplingrate")
// 	outputDeviceLatency := viper.GetDuration("output-device.latency")
// 	outputDeviceChannels := viper.GetString("output-device.channels")

// 	inputDeviceDeviceName := viper.GetString("input-device.device-name")
// 	inputDeviceSamplingrate := viper.GetFloat64("input-device.samplingrate")
// 	inputDeviceLatency := viper.GetDuration("input-device.latency")
// 	inputDeviceChannels := viper.GetString("input-device.channels")

// 	portaudio.Initialize()
// 	defer portaudio.Terminate()

// 	toWireCh := make(chan comms.IOMsg, 20)
// 	toSerializeAudioDataCh := make(chan comms.IOMsg, 20)
// 	toDeserializeAudioDataCh := make(chan []byte, 20)
// 	toDeserializeAudioRespCh := make(chan comms.IOMsg, 10)

// 	evPS := pubsub.New(100)

// 	var wg sync.WaitGroup

// 	settings := comms.MqttSettings{
// 		WaitGroup:  &wg,
// 		Transport:  "tcp",
// 		BrokerURL:  mqttBrokerURL,
// 		BrokerPort: mqttBrokerPort,
// 		ClientID:   mqttClientID,
// 		Username:   mqttUsername,
// 		Password:   mqttPassword,
// 		Topics:     mqttTopics,
// 		ToDeserializeAudioDataCh: toDeserializeAudioDataCh,
// 		ToDeserializeAudioReqCh:  nil,
// 		ToDeserializeAudioRespCh: toDeserializeAudioRespCh,
// 		ToWire:   toWireCh,
// 		Events:   evPS,
// 		LastWill: nil,
// 	}

// 	webserverSettings := webserver.WebServerSettings{
// 		Events:  evPS,
// 		Address: viper.GetString("webui.address"),
// 		Port:    viper.GetInt("webui.port"),
// 	}

// 	player := audio.AudioDevice{
// 		ToWire:           nil,
// 		ToSerialize:      nil,
// 		ToDeserialize:    toDeserializeAudioDataCh,
// 		AudioToWireTopic: serverAudioOutTopic,
// 		Events:           evPS,
// 		WaitGroup:        &wg,
// 		AudioStream: audio.AudioStream{
// 			DeviceName:      outputDeviceDeviceName,
// 			FramesPerBuffer: audioFrameLength,
// 			Samplingrate:    outputDeviceSamplingrate,
// 			Latency:         outputDeviceLatency,
// 			Channels:        audio.GetChannel(outputDeviceChannels),
// 		},
// 	}

// 	recorder := audio.AudioDevice{
// 		ToWire:           toWireCh,
// 		ToSerialize:      toSerializeAudioDataCh,
// 		AudioToWireTopic: serverAudioInTopic,
// 		ToDeserialize:    nil,
// 		Events:           evPS,
// 		WaitGroup:        &wg,
// 		AudioStream: audio.AudioStream{
// 			DeviceName:      inputDeviceDeviceName,
// 			FramesPerBuffer: audioFrameLength,
// 			Samplingrate:    inputDeviceSamplingrate,
// 			Latency:         inputDeviceLatency,
// 			Channels:        audio.GetChannel(inputDeviceChannels),
// 		},
// 	}

// 	wg.Add(3) //mqtt, player, recorder

// 	go webserver.Webserver(webserverSettings)
// 	go events.WatchSystemEvents(evPS)
// 	go audio.PlayerASync(player)
// 	// give the Audio Streams time to setup and start
// 	time.Sleep(time.Millisecond * 150)
// 	go audio.RecorderAsync(recorder)
// 	// give the Audio Streams time to setup and start
// 	time.Sleep(time.Millisecond * 150)
// 	go comms.MqttClient(settings)
// 	// go events.CaptureKeyboard(evPS)

// 	connectionStatusCh := evPS.Sub(events.MqttConnStatus)
// 	reqServerAudioOnCh := evPS.Sub(events.RequestServerAudioOn)
// 	osExitCh := evPS.Sub(events.OsExit)
// 	shutdownCh := evPS.Sub(events.Shutdown)

// 	pingTicker := time.NewTicker(time.Second)

// 	// local states
// 	connectionStatus := comms.DISCONNECTED
// 	serverOnline := false
// 	serverAudioOn := false

// 	for {
// 		select {

// 		// pre shutdown hooks (CTRL-C)
// 		case <-osExitCh:
// 			evPS.Pub(true, events.Shutdown)

// 		// shutdown the application gracefully
// 		case <-shutdownCh:
// 			wg.Wait()
// 			os.Exit(0)

// 		// connection has been established
// 		case ev := <-connectionStatusCh:
// 			connectionStatus = ev.(int)
// 			// evPS.Pub(connectionStatus, events.MqttConnStatus)

// 		// send ping if connected to Broker
// 		case <-pingTicker.C:
// 			if connectionStatus == comms.CONNECTED {
// 				sendPing(mqttClientID, serverRequestTopic, toWireCh)
// 			}

// 		case ev := <-reqServerAudioOnCh:
// 			if connectionStatus == comms.CONNECTED {
// 				if err := sendClientRequest(ev.(bool), serverRequestTopic, toWireCh); err != nil {
// 					fmt.Println(err)
// 				}
// 			}

// 		// responses coming from server
// 		case data := <-toDeserializeAudioRespCh:

// 			msg := sbAudio.ServerResponse{}

// 			err := proto.Unmarshal(data.Data, &msg)
// 			if err != nil {
// 				fmt.Println(err)
// 			}

// 			if msg.Online != nil {
// 				serverOnline = msg.GetOnline()
// 				// fmt.Println("Server Online:", serverOnline)
// 				evPS.Pub(serverOnline, events.ServerOnline)
// 			}

// 			if msg.AudioStream != nil {
// 				serverAudioOn = msg.GetAudioStream()
// 				// fmt.Printf("Server Audio is %t\n", serverAudioOn)
// 				evPS.Pub(serverAudioOn, events.ServerAudioOn)
// 			}

// 			if msg.TxUser != nil {
// 				txUser := msg.GetTxUser()
// 				// fmt.Printf("Server: Current TX User: %s\n", txUser)
// 				evPS.Pub(txUser, events.TxUser)
// 			}

// 			if msg.PingOrigin != nil && msg.Pong != nil {
// 				if msg.GetPingOrigin() == mqttClientID {
// 					pong := time.Unix(0, msg.GetPong())
// 					delta := time.Since(pong)
// 					// fmt.Println("Ping:", delta.Nanoseconds()/1000000, "ms")
// 					evPS.Pub(delta.Nanoseconds(), events.Ping)
// 				}
// 			}
// 		}
// 	}
// }

// func sendClientRequest(audioStreamOn bool, topic string, toWireCh chan comms.IOMsg) error {
// 	req := sbAudio.ClientRequest{}
// 	req.AudioStream = &audioStreamOn
// 	m, err := req.Marshal()
// 	if err != nil {
// 		fmt.Println(err)
// 	} else {
// 		wireMsg := comms.IOMsg{
// 			Topic: topic,
// 			Data:  m,
// 		}
// 		toWireCh <- wireMsg
// 	}

// 	return nil
// }

// func sendPing(user_id, topic string, toWireCh chan comms.IOMsg) {
// 	now := time.Now().UnixNano()

// 	req := sbAudio.ClientRequest{}
// 	req.PingOrigin = &user_id
// 	req.Ping = &now

// 	data, err := req.Marshal()
// 	if err != nil {
// 		fmt.Println(err)
// 	} else {
// 		wireMsg := comms.IOMsg{
// 			Topic: topic,
// 			Data:  data,
// 		}
// 		toWireCh <- wireMsg
// 	}
// }

// type parmError struct {
// 	parm string
// 	msg  string
// }

// func (e *parmError) Error() string {
// 	return fmt.Sprintf("parameter error (%s): %s", e.parm, e.msg)
// }
