package trx

import (
	"fmt"
	"log"
	"sync"

	"github.com/dh1tw/remoteAudio/audio/pbWriter"

	"github.com/dh1tw/remoteAudio/audio/pbReader"

	"github.com/dh1tw/remoteAudio/audio/chain"
	"github.com/dh1tw/remoteAudio/proxy"
	"github.com/micro/go-micro/broker"
)

type Trx struct {
	sync.RWMutex
	rx          *chain.Chain
	tx          *chain.Chain
	fromNetwork *pbReader.PbReader
	toNetwork   *pbWriter.PbWriter
	broker      broker.Broker
	servers     map[string]*proxy.AudioServer
	rxAudioSub  broker.Subscriber
	curServer   *proxy.AudioServer
}

type Options struct {
	Rx          *chain.Chain
	Tx          *chain.Chain
	FromNetwork *pbReader.PbReader
	ToNetwork   *pbWriter.PbWriter
	Broker      broker.Broker
}

func NewTrx(opts Options) (*Trx, error) {

	if opts.Rx == nil {
		return nil, fmt.Errorf("rx variable is nil")
	}

	if opts.Tx == nil {
		return nil, fmt.Errorf("tx variable is nil")
	}
	if opts.FromNetwork == nil {
		return nil, fmt.Errorf("fromNetwork sink is nil")
	}
	if opts.ToNetwork == nil {
		return nil, fmt.Errorf("toNetwork sink is nil")
	}

	trx := &Trx{
		rx:          opts.Rx,
		tx:          opts.Tx,
		fromNetwork: opts.FromNetwork,
		toNetwork:   opts.ToNetwork,
		servers:     make(map[string]*proxy.AudioServer),
	}

	trx.toNetwork.SetToWireCb(trx.toWireCb)

	return trx, nil
}

func (x *Trx) AddServer(asvr *proxy.AudioServer) {
	x.Lock()
	defer x.Unlock()

	if asvr == nil {
		return
	}
	_, ok := x.servers[asvr.Name()]
	if ok {
		return
	}
	x.servers[asvr.Name()] = asvr
}

func (x *Trx) RemoveServer(asvr *proxy.AudioServer) {
	x.Lock()
	defer x.Unlock()

	if asvr == nil {
		return
	}
	_, ok := x.servers[asvr.Name()]
	if !ok {
		return
	}
	delete(x.servers, asvr.Name())
}

func (x *Trx) SelectServer(name string) error {
	x.Lock()
	defer x.Unlock()

	newSvr, ok := x.servers[name]
	if !ok {
		return fmt.Errorf("unknown audio server: %v", name)
	}

	if err := x.rxAudioSub.Unsubscribe(); err != nil {
		return fmt.Errorf("select server unsubscribe: %v", err)
	}

	x.curServer = newSvr

	sub, err := x.broker.Subscribe(newSvr.RxAddress(), x.fromWireCb)
	if err != nil {
		return fmt.Errorf("select server subscribe: %v", err)
	}

	x.rxAudioSub = sub

	// publish to new topic

	return nil
}

func (x *Trx) SetRxVolume(vol float32) error {
	x.Lock()
	defer x.Unlock()

	speaker, _, err := x.rx.Sinks.Sink("speaker")
	if err != nil {
		return err
	}
	speaker.SetVolume(vol)
	return nil
}

func (x *Trx) GetRxVolume() (float32, error) {
	x.Lock()
	defer x.Unlock()

	speaker, _, err := x.rx.Sinks.Sink("speaker")
	if err != nil {
		return 0, err
	}
	return speaker.Volume(), nil
}

func (x *Trx) SetTxVolume(vol float32) error {
	x.Lock()
	defer x.Unlock()

	toNetwork, _, err := x.tx.Sinks.Sink("toNetwork")
	if err != nil {
		return err
	}
	toNetwork.SetVolume(vol)
	return nil
}

func (x *Trx) GetTxVolume() (float32, error) {
	x.Lock()
	defer x.Unlock()

	toNetwork, _, err := x.tx.Sinks.Sink("toNetwork")
	if err != nil {
		return 0, err
	}
	return toNetwork.Volume(), nil
}

func (x *Trx) SetTxState(on bool) error {
	x.Lock()
	defer x.Unlock()

	return x.tx.Sinks.EnableSink("toNetwork", true)
}

func (x *Trx) SetRxState(on bool) error {
	x.Lock()
	defer x.Unlock()
	if x.curServer == nil {
		return fmt.Errorf("no audio server selected")
	}

	var err error

	if on {
		err = x.curServer.StartRxStream()
	} else {
		err = x.curServer.StopRxStream()
	}

	return err
}

func (x *Trx) GetTxState() (bool, error) {
	x.Lock()
	defer x.Unlock()

	_, state, err := x.tx.Sinks.Sink("toNetwork")
	if err != nil {
		return false, err
	}
	return state, nil
}

func (x *Trx) GetRxState() (bool, error) {
	x.Lock()
	defer x.Unlock()

	if x.curServer == nil {
		return false, fmt.Errorf("no audio server selected")
	}

	return x.curServer.RxOn(), nil
}

func (x *Trx) GetTxUser() (string, error) {
	x.Lock()
	defer x.Unlock()

	if x.curServer == nil {
		return "", fmt.Errorf("no audio server selected")
	}

	return x.curServer.TxUser(), nil
}

func (x *Trx) fromWireCb(pub broker.Publication) error {
	x.RLock()
	defer x.RUnlock()

	if x.fromNetwork == nil {
		return nil
	}
	return x.fromNetwork.Enqueue(pub.Message().Body)
}

func (x *Trx) toWireCb(data []byte) {
	x.RLock()
	defer x.RUnlock()
	// Callback which is called by pbWriter to push the audio
	// packets to the network
	msg := &broker.Message{
		Body: data,
	}

	err := x.broker.Publish(x.curServer.TxAddress(), msg)
	if err != nil {
		log.Println(err)
	}
}
