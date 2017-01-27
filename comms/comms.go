package comms

import (
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
	ToDeserializeAudioDataCh chan []byte
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
	Data       []byte
	Raw        []float32
	Topic      string
	Retain     bool
	Qos        byte
	MQTTts     time.Time
	EnqueuedTs time.Time
}

const (
	DISCONNECTED = 0
	CONNECTED    = 1
)

func MqttClient(s MqttSettings) {

	defer s.WaitGroup.Done()

	// mqtt.DEBUG = log.New(os.Stderr, "DEBUG - ", log.LstdFlags)
	// mqtt.CRITICAL = log.New(os.Stderr, "CRITICAL - ", log.LstdFlags)
	// mqtt.WARN = log.New(os.Stderr, "WARN - ", log.LstdFlags)
	// mqtt.ERROR = log.New(os.Stderr, "ERROR - ", log.LstdFlags)

	shutdownCh := s.Events.Sub(events.Shutdown)
	forwardAudioCh := s.Events.Sub(events.ForwardAudio)

	forwardAudio := false

	var msgHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		if strings.Contains(msg.Topic(), "audio/audio") {

			if forwardAudio {
				s.ToDeserializeAudioDataCh <- msg.Payload()[:len(msg.Payload())]
			}

		} else if strings.Contains(msg.Topic(), "request") {

			audioRespMsg := IOMsg{
				Data: msg.Payload()[:len(msg.Payload())],
			}
			s.ToDeserializeAudioReqCh <- audioRespMsg

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
		log.Printf("Connected to MQTT Broker %s:%d\n", s.BrokerURL, s.BrokerPort)

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
			if client.IsConnected() {
				client.Disconnect(0)
			}
			return
		case msg := <-s.ToWire:
			token := client.Publish(msg.Topic, msg.Qos, msg.Retain, msg.Data)
			token.Wait()

		//indicates if audio data should be forwarded for decoding & play
		case ev := <-forwardAudioCh:
			forwardAudio = ev.(bool)
		}
	}
}
