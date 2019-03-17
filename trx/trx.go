package trx

import (
	"fmt"
	"log"
	"sync"

	"github.com/dh1tw/remoteAudio/audio/sinks/pbWriter"

	"github.com/dh1tw/remoteAudio/audio/sources/pbReader"

	"github.com/dh1tw/remoteAudio/audio/chain"
	"github.com/dh1tw/remoteAudio/proxy"
	"github.com/micro/go-micro/broker"
)

// Trx is a data structure which holds the components needed for a 2-way
// radio. It holds the available audio servers, the communication means,
// the audio rx and tx chains etc.
type Trx struct {
	sync.RWMutex
	rx                   *chain.Chain
	tx                   *chain.Chain
	fromNetwork          *pbReader.PbReader
	toNetwork            *pbWriter.PbWriter
	broker               broker.Broker
	servers              map[string]*proxy.AudioServer
	rxAudioSub           broker.Subscriber
	curServer            *proxy.AudioServer
	notifyServerChangeCb func()
}

// Options is the data structure holding the values used for instantiating
// a Trx object. This struct has to be provided the the object constructor.
type Options struct {
	Rx          *chain.Chain
	Tx          *chain.Chain
	FromNetwork *pbReader.PbReader
	ToNetwork   *pbWriter.PbWriter
	Broker      broker.Broker
}

// NewTrx is the constructor method of a Trx object.
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
	if opts.Broker == nil {
		return nil, fmt.Errorf("broker is nil")
	}

	trx := &Trx{
		rx:          opts.Rx,
		tx:          opts.Tx,
		fromNetwork: opts.FromNetwork,
		toNetwork:   opts.ToNetwork,
		broker:      opts.Broker,
		servers:     make(map[string]*proxy.AudioServer),
	}

	trx.toNetwork.SetToWireCb(trx.toWireCb)

	return trx, nil
}

// SetNotifyServerChangeCb allows to set a callback which get's executed
// when a remote audio server changes / disappears.
func (x *Trx) SetNotifyServerChangeCb(f func()) {
	x.Lock()
	defer x.Unlock()
	x.notifyServerChangeCb = f
}

// AddServer adds a remote audio server, represented through a proxy object.
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

	asvr.SetNotifyCb(x.onAudioServersChanged)
	go x.onAudioServersChanged()

	log.Println("added audio server", asvr.Name())
}

// onAudioServersChanged will execute a callback to inform the parent
// application that a remote audio server has changed.
func (x *Trx) onAudioServersChanged() {
	x.RLock()
	defer x.RUnlock()
	if x.notifyServerChangeCb != nil {
		go x.notifyServerChangeCb()
	}
}

// Server returns a particular AudioServer. If no
// AudioServer exists with that name, (nil, false) will be returned.
func (x *Trx) Server(name string) (*proxy.AudioServer, bool) {
	x.RLock()
	defer x.RUnlock()

	svr, ok := x.servers[name]
	return svr, ok
}

// Servers returns a list with the names of the registered
// audio servers.
func (x *Trx) Servers() []string {
	x.RLock()
	defer x.RUnlock()

	list := []string{}
	for _, aServer := range x.servers {
		list = append(list, aServer.Name())
	}
	return list
}

// RemoveServer removes a remote audio server from the Trx.
func (x *Trx) RemoveServer(asName string) error {
	x.Lock()
	defer x.Unlock()

	as, ok := x.servers[asName]
	if !ok {
		return fmt.Errorf("unable to remove unknown audio server: %v", asName)
	}

	delete(x.servers, asName)

	if as.Name() == x.curServer.Name() && len(x.servers) > 0 {
		for _, svr := range x.servers {
			go x.SelectServer(svr.Name()) // must be async to avoid deadlock
		}
	} else if len(x.servers) == 0 {
		x.curServer = nil
		if err := x.tx.Enable(false); err != nil {
			log.Println(err) // better fatal?
		}
	}

	log.Println("removed audio server", as.Name())

	as.Close()
	// emit event
	go x.onAudioServersChanged()

	return nil
}

// SelectServer selects a particular remote audio server from which the
// audio will be received / sent to.
func (x *Trx) SelectServer(name string) error {
	x.Lock()
	defer x.Unlock()

	newSvr, ok := x.servers[name]
	if !ok {
		return fmt.Errorf("unknown audio server: %v", name)
	}

	if x.rxAudioSub != nil {
		if err := x.rxAudioSub.Unsubscribe(); err != nil {
			return fmt.Errorf("select server unsubscribe: %v", err)
		}
	}

	x.curServer = newSvr

	sub, err := x.broker.Subscribe(newSvr.RxAddress(), x.fromWireCb)
	if err != nil {
		return fmt.Errorf("SelectServer subscribe: %v", err)
	}

	x.rxAudioSub = sub

	return nil
}

// SelectedServer returns the name of the currently selected Audio Server.
func (x *Trx) SelectedServer() string {
	x.RLock()
	defer x.RUnlock()

	if x.curServer != nil {
		return x.curServer.Name()
	}
	return ""
}

// SetRxVolume sets the volume of the local speakers.
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

// RxVolume returns the currently set volume for the local speakers.
func (x *Trx) RxVolume() (float32, error) {
	x.Lock()
	defer x.Unlock()

	speaker, _, err := x.rx.Sinks.Sink("speaker")
	if err != nil {
		return 0, err
	}
	return speaker.Volume(), nil
}

// SetTxVolume sets the volume of the audio sent to the remote audio server.
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

// TxVolume returns the current volume level for the audio sent to the remote audio server.
func (x *Trx) TxVolume() (float32, error) {
	x.Lock()
	defer x.Unlock()

	toNetwork, _, err := x.tx.Sinks.Sink("toNetwork")
	if err != nil {
		return 0, err
	}
	return toNetwork.Volume(), nil
}

// SetTxState turns on/off the audio stream sent to the remote audio server.
// It can be considered PTT (Push To Talk)
func (x *Trx) SetTxState(on bool) error {
	x.Lock()
	defer x.Unlock()

	if x.curServer == nil {
		return nil
	}

	if on {
		return x.tx.Enable(true)
	}
	return x.tx.Enable(false)
}

// SetRxState turns on/off the audio stream sent from the remote audio server.
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

// TxState returns a boolean if audio is currently sent to the remote
// audio server, or not.
func (x *Trx) TxState() (bool, error) {
	x.Lock()
	defer x.Unlock()

	_, state, err := x.tx.Sinks.Sink("toNetwork")
	if err != nil {
		return false, err
	}
	return state, nil
}

// RxState returns a boolean if the remote audio server is streaming
// audio or not.
func (x *Trx) RxState() (bool, error) {
	x.Lock()
	defer x.Unlock()

	if x.curServer == nil {
		return false, fmt.Errorf("no audio server selected")
	}

	return x.curServer.RxOn(), nil
}

// TxUser returns the current user from which the remote audio server
// is receiving audio. If nobody is transmitting / sending audio to the
// remote audio server, an empty string will be returned.
func (x *Trx) TxUser() (string, error) {
	x.Lock()
	defer x.Unlock()

	if x.curServer == nil {
		return "", fmt.Errorf("no audio server selected")
	}

	return x.curServer.TxUser(), nil
}

// fromWireCb is a callback that is executed when audio is received
// from the network. It will typically then enqueue the received data
// into an audio source / chain.
func (x *Trx) fromWireCb(pub broker.Publication) error {
	x.RLock()
	defer x.RUnlock()

	if x.fromNetwork == nil {
		return nil
	}
	return x.fromNetwork.Enqueue(pub.Message().Body)
}

// toWireCb is a callback that is executed when audio is ready to
// be sent to the audio server. Typically this callback is called from
// an audio sync (e.g. pbWriter).
func (x *Trx) toWireCb(data []byte) {
	// Callback which is called by pbWriter to push the audio
	// packets to the network
	// NO MUTEX! (causes deadlock)
	msg := &broker.Message{
		Body: data,
	}

	if x.curServer == nil {
		return
	}

	err := x.broker.Publish(x.curServer.TxAddress(), msg)
	if err != nil {
		log.Fatal("toWireCb:", err)
	}
}
