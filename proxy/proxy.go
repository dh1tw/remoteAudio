package proxy

import (
	"context"
	"fmt"
	"sync"

	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
	"github.com/gogo/protobuf/proto"
	"github.com/micro/go-micro/broker"
	"github.com/micro/go-micro/client"
)

type AudioServer struct {
	sync.RWMutex
	name         string
	serviceName  string
	client       client.Client
	rpc          sbAudio.ServerClient
	stateSub     broker.Subscriber
	rxAddress    string
	txAddress    string
	stateAddress string
	rxOn         bool
	txUser       string
}

func NewAudioServer(name string, client client.Client, opts ...Option) (*AudioServer, error) {

	serviceName := fmt.Sprintf("shackbus.radio.%s.audio", name)

	as := &AudioServer{
		name:         name,
		serviceName:  serviceName,
		client:       client,
		rxAddress:    fmt.Sprintf("%s.rx", serviceName),
		txAddress:    fmt.Sprintf("%s.tx", serviceName),
		stateAddress: fmt.Sprintf("%s.state", serviceName),
		txUser:       "",
	}

	as.rpc = sbAudio.NewServerClient(as.serviceName, as.client)

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

	return as, nil
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
	return err
}

func (as *AudioServer) StopRxStream() error {
	_, err := as.rpc.StopStream(context.Background(), &sbAudio.None{})
	return err
}

func (as *AudioServer) TxUser() string {
	as.RLock()
	defer as.RUnlock()
	return as.txUser
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
