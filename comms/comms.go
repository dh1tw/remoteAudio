package comms

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cskr/pubsub"
	"github.com/dh1tw/remoteAudio/events"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type MqttSettings struct {
	WaitGroup                *sync.WaitGroup
	Transport                string
	BrokerURL                string
	BrokerPort               int
	ClientID                 string
	Topics                   []string
	ToDeserializeAudioDataCh chan IOMsg
	ToDeserializeAudioReqCh  chan IOMsg
	ToDeserializeAudioRespCh chan IOMsg
	ToWire                   chan IOMsg
	Events                   *pubsub.PubSub
	LastWill                 *LastWill
}

// LastWill defines the LastWill for MQTT. The LastWill will be
// submitted to the broker on connection and will be published
// on Disconnect.
type LastWill struct {
	Topic  string
	Data   []byte
	Qos    byte
	Retain bool
}

// IOMsg is a struct used internally which either originates from or
// will be send to the wire
type IOMsg struct {
	Data   []byte
	Raw    []float32
	Topic  string
	Retain bool
	Qos    byte
}

const (
	DISCONNECTED = 0
	CONNECTED    = 1
)

func MqttClient(s MqttSettings) {

	// mqtt.DEBUG = log.New(os.Stderr, "DEBUG - ", log.LstdFlags)
	// mqtt.CRITICAL = log.New(os.Stderr, "CRITICAL - ", log.LstdFlags)
	// mqtt.WARN = log.New(os.Stderr, "WARN - ", log.LstdFlags)
	// mqtt.ERROR = log.New(os.Stderr, "ERROR - ", log.LstdFlags)

	shutdownCh := s.Events.Sub(events.Shutdown)

	var msgHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		if strings.Contains(msg.Topic(), "audio/audio") {
			audioMsg := IOMsg{
				Topic: msg.Topic(),
				Data:  msg.Payload()[:len(msg.Payload())],
			}
			fmt.Println("NETWORK", time.Now().Format(time.StampMilli))
			s.ToDeserializeAudioDataCh <- audioMsg

		} else if strings.Contains(msg.Topic(), "request") {
			audioReqMsg := IOMsg{
				Data: msg.Payload()[:len(msg.Payload())],
			}
			s.ToDeserializeAudioReqCh <- audioReqMsg

		} else if strings.Contains(msg.Topic(), "response") {
			audioRespMsg := IOMsg{
				Data: msg.Payload()[:len(msg.Payload())],
			}
			s.ToDeserializeAudioRespCh <- audioRespMsg
		}
	}

	var connectionLostHandler = func(client mqtt.Client, err error) {
		log.Println("Connection lost to MQTT Broker; Reason:", err)
		s.Events.Pub(DISCONNECTED, events.MqttConnStatus)
	}

	// since we use SetCleanSession we have to subscribe on each
	// connect or reconnect to the channels
	var onConnectHandler = func(client mqtt.Client) {
		log.Println("Connected to MQTT Broker ")

		// Subscribe to Task Topics
		for _, topic := range s.Topics {
			if token := client.Subscribe(topic, 0, nil); token.Wait() &&
				token.Error() != nil {
				log.Println(token.Error())
			}
		}
		s.Events.Pub(CONNECTED, events.MqttConnStatus)
	}

	opts := mqtt.NewClientOptions().AddBroker(s.Transport + "://" + s.BrokerURL + ":" + strconv.Itoa(s.BrokerPort))
	opts.SetClientID(s.ClientID)
	opts.SetDefaultPublishHandler(msgHandler)
	opts.SetKeepAlive(time.Second * 5)
	opts.SetMaxReconnectInterval(time.Second)
	opts.SetCleanSession(true)
	opts.SetOnConnectHandler(onConnectHandler)
	opts.SetConnectionLostHandler(connectionLostHandler)
	opts.SetAutoReconnect(true)

	if s.LastWill != nil {
		opts.SetBinaryWill(s.LastWill.Topic, s.LastWill.Data, s.LastWill.Qos, s.LastWill.Retain)
	}

	client := mqtt.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Println(token.Error())
	}

	for {
		select {
		case <-shutdownCh:
			log.Println("Disconnecting from MQTT Broker")
			client.Disconnect(0)
			s.WaitGroup.Done()
			return
		case msg := <-s.ToWire:
			token := client.Publish(msg.Topic, msg.Qos, msg.Retain, msg.Data)
			token.Wait()
		}
	}
}
