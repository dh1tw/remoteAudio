package audioserver

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dh1tw/remoteAudio/audio/chain"
	"github.com/dh1tw/remoteAudio/audio/sources/pbReader"
	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
	"github.com/gogo/protobuf/proto"
	micro "github.com/micro/go-micro"
	"github.com/micro/go-micro/broker"
)

// AudioServer is implementing the RPC shackbus.audio.server using the
// micro microservice framework. Audioserver is also a convenience object
// which contains all the long living variables & objects of this application.
type AudioServer struct {
	sync.RWMutex
	name         string
	service      micro.Service
	broker       broker.Broker
	rx           *chain.Chain
	tx           *chain.Chain
	fromNetwork  *pbReader.PbReader
	rxAudioTopic string
	txAudioTopic string
	stateTopic   string
	txAudioSub   broker.Subscriber
	rxOn         bool
	txUser       string
	serverIndex  int
	lastPing     time.Time
}

type Option func(*Options)

type Options struct {
	ServiceName string
	ServerIndex int
	Service     micro.Service
	Broker      broker.Broker
	RxChain     *chain.Chain
	TxChain     *chain.Chain
	FromNetwork *pbReader.PbReader
}

func Service(s micro.Service) Option {
	return func(args *Options) {
		args.Service = s
	}
}

func ServiceName(name string) Option {
	return func(args *Options) {
		args.ServiceName = name
	}
}

func Broker(b broker.Broker) Option {
	return func(args *Options) {
		args.Broker = b
	}
}

func RxChain(r *chain.Chain) Option {
	return func(args *Options) {
		args.RxChain = r
	}
}

func TxChain(t *chain.Chain) Option {
	return func(args *Options) {
		args.TxChain = t
	}
}

func Index(i int) Option {
	return func(args *Options) {
		args.ServerIndex = i
	}
}

func FromNetwork(pbr *pbReader.PbReader) Option {
	return func(args *Options) {
		args.FromNetwork = pbr
	}
}

// NewAudioServer is constructor method of an AudioServer.
func NewAudioServer(opts ...Option) (*AudioServer, error) {

	as := &AudioServer{
		lastPing:    time.Now(),
		serverIndex: 0,
	}

	options := Options{}

	for _, option := range opts {
		option(&options)
	}

	as.name = options.ServiceName
	as.rxAudioTopic = options.ServiceName + ".rx"
	as.txAudioTopic = options.ServiceName + ".tx"
	as.stateTopic = options.ServiceName + ".state"
	as.serverIndex = options.ServerIndex
	as.fromNetwork = options.FromNetwork

	// connect the broker
	if err := as.broker.Connect(); err != nil {
		return nil, fmt.Errorf("broker: %v", err)
	}

	// subscribe to the audio topic and enqueue the raw data into the pbReader
	sub, err := as.broker.Subscribe(as.txAudioTopic, as.enqueueFromWire)
	if err != nil {
		return nil, fmt.Errorf("subscribe: %v", err)
	}

	as.txAudioSub = sub

	as.rx.Sources.SetSource("radioAudio")

	// stream immediately audio from the network to the radio
	as.tx.Sources.SetSource("fromNetwork")
	if err := as.tx.Enable(true); err != nil {
		return nil, err
	}

	// when no ping is received, turn of the audio stream
	go as.checkTimeout()

	return as, nil
}

func (as *AudioServer) enqueueFromWire(pub broker.Publication) error {
	if as.fromNetwork == nil {
		return nil
	}
	return as.fromNetwork.Enqueue(pub.Message().Body)
}

// Callback which is called by pbWriter to push the audio
// packets to the network
func (as *AudioServer) ToWireCb(data []byte) {

	if as.broker == nil {
		log.Println("sendState: broker not set") // better Fatal?
	}

	msg := &broker.Message{
		Body: data,
	}

	err := as.broker.Publish(as.rxAudioTopic, msg)
	if err != nil {
		log.Println(err) // better fatal?
	}
}

func (as *AudioServer) sendState() error {
	as.RLock()
	defer as.RUnlock()

	if as.broker == nil {
		return fmt.Errorf("sendState: broker not set")
	}

	state := sbAudio.State{
		RxOn:   as.rxOn,
		TxUser: as.txUser,
	}

	data, err := proto.Marshal(&state)
	if err != nil {
		return err
	}

	msg := &broker.Message{
		Body: data,
	}

	return as.broker.Publish(as.stateTopic, msg)
}

func (as *AudioServer) GetCapabilities(ctx context.Context, in *sbAudio.None, out *sbAudio.Capabilities) error {
	as.RLock()
	defer as.RUnlock()
	out.Name = as.name
	out.RxStreamAddress = as.rxAudioTopic
	out.TxStreamAddress = as.txAudioTopic
	out.StateUpdatesAddress = as.stateTopic
	out.Index = int32(as.serverIndex)
	return nil
}

func (as *AudioServer) GetState(ctx context.Context, in *sbAudio.None, out *sbAudio.State) error {
	rxOn, txUser, err := as.getState()
	if err != nil {
		return err
	}
	out.RxOn = rxOn
	out.TxUser = txUser
	return nil
}

func (as *AudioServer) StartStream(ctx context.Context, in, out *sbAudio.None) error {

	if err := as.rx.Enable(true); err != nil {
		log.Println("StartStream:", err)
		return err
	}

	as.Lock()
	as.rxOn = true
	as.Unlock()

	if err := as.sendState(); err != nil {
		log.Println("StartStream:", err)
		return err
	}
	return nil
}

func (as *AudioServer) StopStream(ctx context.Context, in, out *sbAudio.None) error {

	if err := as.rx.Enable(false); err != nil {
		log.Println("StopStream:", err)
		return err
	}

	as.Lock()
	as.rxOn = false
	as.Unlock()

	if err := as.sendState(); err != nil {
		log.Println("StopStream:", err)
		return err
	}
	return nil
}

func (as *AudioServer) Ping(ctx context.Context, in, out *sbAudio.PingPong) error {
	out.Ping = in.Ping
	as.Lock()
	defer as.Unlock()
	as.lastPing = time.Now()
	return nil
}

func (as *AudioServer) getState() (bool, string, error) {
	as.RLock()
	defer as.RUnlock()
	_, rxOn, err := as.rx.Sinks.Sink("toNetwork")
	if err != nil {
		return false, "", err
	}
	return rxOn, as.txUser, nil
}

func (as *AudioServer) checkTimeout() {

	ticker := time.NewTicker(time.Minute)

	for {
		<-ticker.C
		as.RLock()
		if time.Since(as.lastPing) > time.Duration(time.Minute) {
			if err := as.rx.Enable(false); err != nil {
				log.Println("checkTimeout: ", err)
			}
		}
		as.RUnlock()
	}
}
