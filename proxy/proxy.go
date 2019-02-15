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

type AudioServer struct {
	sync.RWMutex
	name           string
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
// oneway latency (ping/2) in milli seconds.
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

	res := int((now - pong.Ping) / 2)
	return res, nil
}

func (as *AudioServer) SetNotifyCb(f func()) {
	as.Lock()
	defer as.Unlock()
	as.notifyChangeCb = f
}

func (as *AudioServer) Name() string {
	as.RLock()
	defer as.RUnlock()
	return as.name
}

func (as *AudioServer) ServiceName() string {
	as.RLock()
	defer as.RUnlock()
	return as.serviceName
}

func (as *AudioServer) RxOn() bool {
	as.RLock()
	defer as.RUnlock()
	return as.rxOn
}

func (as *AudioServer) RxAddress() string {
	as.RLock()
	defer as.RUnlock()
	return as.rxAddress
}

func (as *AudioServer) TxAddress() string {
	as.RLock()
	defer as.RUnlock()
	return as.txAddress
}

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

func (as *AudioServer) TxUser() string {
	as.RLock()
	defer as.RUnlock()
	return as.txUser
}

func (as *AudioServer) Latency() int {
	as.RLock()
	defer as.RUnlock()
	return as.latency
}

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
	return nil
}
