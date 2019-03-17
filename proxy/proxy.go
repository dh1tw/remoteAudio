package proxy

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
	"github.com/gogo/protobuf/proto"
	"github.com/micro/go-micro/broker"
	"github.com/micro/go-micro/client"
)

// AudioServer is a local proxy object respresenting a remote Audio server. It can
// be considered an abstraction layer so you don't have to take care of
// sending and receiving messages to the remote audio server. This proxy
// implementation is based on the micro.mu microservice framework.
type AudioServer struct {
	sync.RWMutex
	name           string
	index          int //static index for displaying several servers consistenly in the GUI
	serviceName    string
	client         client.Client
	rpc            sbAudio.ServerService
	stateSub       broker.Subscriber
	rxAddress      string
	txAddress      string
	stateAddress   string
	rxOn           bool
	txUser         string
	latency        int
	notifyChangeCb func()
	closePing      chan struct{}
	doneCh         chan struct{}
	doneOnce       sync.Once
}

// NewAudioServer is the constructor for the Audioserver proxy. The communication
// with the remote audio server is done through a micro client. In case the
// object disappears the doneCh will be closed.
func NewAudioServer(name string, client client.Client, doneCh chan struct{}, opts ...Option) (*AudioServer, error) {

	serviceName := fmt.Sprintf("shackbus.radio.%s.audio", name)

	as := &AudioServer{
		name:         name,
		serviceName:  serviceName,
		client:       client,
		rxAddress:    fmt.Sprintf("%s.rx", serviceName),
		txAddress:    fmt.Sprintf("%s.tx", serviceName),
		stateAddress: fmt.Sprintf("%s.state", serviceName),
		txUser:       "",
		closePing:    make(chan struct{}),
		doneCh:       doneCh,
	}

	as.rpc = sbAudio.NewServerService(as.serviceName, as.client)

	if err := as.getCapabilities(); err != nil {
		return nil, err
	}
	if err := as.getState(); err != nil {
		return nil, err
	}

	sub, err := as.client.Options().Broker.Subscribe(as.stateAddress, as.stateUpdateCb)
	if err != nil {
		return nil, err
	}

	as.stateSub = sub

	// start a go routine to ping our service every second
	// for monitoring the latency to the server.
	go func() {
		for {
			select {
			case <-time.After(time.Second * 3):

				ping, err := as.ping()

				if err != nil {
					log.Println("unable to ping service", as.Name())
					as.closeDone()
					return
				}
				as.Lock()
				as.latency = ping
				as.Unlock()
				go as.notifyChangeCb()
			case <-as.closePing:
				return
			}
		}
	}()

	return as, nil
}

// the doneCh must be closed through this function to avoid
// multiple times closing this channel. Closing the doneCh signals the
// application that this object can be disposed.
func (as *AudioServer) closeDone() {
	as.doneOnce.Do(func() { close(as.doneCh) })
}

// Close shutsdown this object and all associated go routines so that
// it can be garbage collected.
func (as *AudioServer) Close() {
	as.Lock()
	defer as.Unlock()

	as.stateSub.Unsubscribe()
	close(as.closePing)
	as.rpc = nil
	as.client = nil
	as.notifyChangeCb = nil
	as.closeDone()
}

// ping performs a ping request to the audio server and returns the
// latency (ping / 2-way) in milliseconds.
func (as *AudioServer) ping() (int, error) {

	ping := time.Now().UnixNano() / int64(time.Millisecond)

	pingMsg := sbAudio.PingPong{
		Ping: ping,
	}

	pong, err := as.rpc.Ping(context.Background(), &pingMsg)
	if err != nil {
		return 0, err
	}

	now := time.Now().UnixNano() / int64(time.Millisecond)

	res := int(now - pong.Ping)
	return res, nil
}

// SetNotifyCb sets a callback which will be executed whenever the
// state of the server changes.
func (as *AudioServer) SetNotifyCb(f func()) {
	as.Lock()
	defer as.Unlock()
	as.notifyChangeCb = f
}

// Name returns the name of the remote audio server
func (as *AudioServer) Name() string {
	as.RLock()
	defer as.RUnlock()
	return as.name
}

// Index returns the static index of the remote audio server. This is only
// needed for maintaining a consistent order in a GUI if several audio
// servers are available
func (as *AudioServer) Index() int {
	as.RLock()
	defer as.RUnlock()
	return as.index
}

// ServiceName returns the fully qualified service name of the remote audio server
func (as *AudioServer) ServiceName() string {
	as.RLock()
	defer as.RUnlock()
	return as.serviceName
}

// RxOn returns a boolean which indicates the the remote audio server is
// streaming audio.
func (as *AudioServer) RxOn() bool {
	as.RLock()
	defer as.RUnlock()
	return as.rxOn
}

// RxAddress returns the address on which the remote audio server is sending
// out the audio.
func (as *AudioServer) RxAddress() string {
	as.RLock()
	defer as.RUnlock()
	return as.rxAddress
}

// TxAddress returns the address on which the remote audio server is listening
// for incoming audio to be transmitted.
func (as *AudioServer) TxAddress() string {
	as.RLock()
	defer as.RUnlock()
	return as.txAddress
}

// StartRxStream tells the remote audio server to start streaming audio.
func (as *AudioServer) StartRxStream() error {
	_, err := as.rpc.StartStream(context.Background(), &sbAudio.None{})
	if err != nil {
		return err
	}
	as.Lock()
	as.rxOn = true
	as.Unlock()
	return nil
}

// StopRxStream tells the remote audio server to stop streaming audio.
func (as *AudioServer) StopRxStream() error {
	_, err := as.rpc.StopStream(context.Background(), &sbAudio.None{})
	if err != nil {
		return err
	}
	as.Lock()
	as.rxOn = false
	as.Unlock()
	return nil
}

// TxUser returns the current user transmitting through the remote audio server.
// In case nobody is transmitting, an empty string will be returned.
func (as *AudioServer) TxUser() string {
	as.RLock()
	defer as.RUnlock()
	return as.txUser
}

// Latency returns the ping (2-way) latency to the remote audio server.
func (as *AudioServer) Latency() int {
	as.RLock()
	defer as.RUnlock()
	return as.latency
}

// stateUpdateCb decodes a protobuf coming from the micro broker and
// notifies the parent application through a callback.
func (as *AudioServer) stateUpdateCb(msg broker.Publication) error {

	newState := sbAudio.State{}

	if err := proto.Unmarshal(msg.Message().Body, &newState); err != nil {
		return err
	}
	as.Lock()
	defer as.Unlock()

	as.rxOn = newState.GetRxOn()
	as.txUser = newState.GetTxUser()

	if as.notifyChangeCb != nil {
		go as.notifyChangeCb()
	}

	// TBD check if something has changed and notify subcribers

	return nil
}

// getState queries the remote audio server to retrieve it's state.
func (as *AudioServer) getState() error {
	state, err := as.rpc.GetState(context.Background(), &sbAudio.None{})
	if err != nil {
		return fmt.Errorf("getState: %v", err)
	}
	as.Lock()
	defer as.Unlock()
	as.rxOn = state.RxOn
	as.txUser = state.TxUser

	return nil
}

// getCapabilities queries the remote audio server to retrieve it's
// capabilities
func (as *AudioServer) getCapabilities() error {
	caps, err := as.rpc.GetCapabilities(context.Background(), &sbAudio.None{})
	if err != nil {
		return fmt.Errorf("getCapabilities: %v", err)
	}

	as.Lock()
	defer as.Unlock()

	as.rxAddress = caps.GetRxStreamAddress()
	if len(as.rxAddress) == 0 {
		return fmt.Errorf("getCapabilities: RxStreamAddress empty")
	}
	as.txAddress = caps.GetTxStreamAddress()
	if len(as.txAddress) == 0 {
		return fmt.Errorf("getCapabilities: TxStreamAddress empty")
	}
	as.stateAddress = caps.GetStateUpdatesAddress()
	if len(as.stateAddress) == 0 {
		return fmt.Errorf("getCapabilities: StateUpdatesAddress empty")
	}
	as.index = int(caps.GetIndex())

	return nil
}
