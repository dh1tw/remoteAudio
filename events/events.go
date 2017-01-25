package events

import (
	"os"
	"os/signal"

	"github.com/cskr/pubsub"
)

// Event channel names used for event Pubsub

// internal
const (
	MqttConnStatus    = "mqttConnStatus"    // int
	RecordAudioOn     = "recordAudio"       // bool
	NewAudioFrameSize = "newaudioframesize" // int
	ForwardAudio      = "forwardAudio"      //bool
	Shutdown          = "shutdown"          // bool
	SetVolume         = "setVolume"         // int
	OsExit            = "osExit"            // bool
)

// for message handling
const (
	ServerOnline         = "serverOnline" //bool
	ServerAudioOn        = "audioOn"      // bool
	RequestServerAudioOn = "reqAudioOn"   // bool
	TxUser               = "txUser"       // string
	Ping                 = "ping"         // int64
)

func WatchSystemEvents(evPS *pubsub.PubSub) {

	// Channel to handle OS signals
	osSignals := make(chan os.Signal, 1)

	//subscribe to os.Interrupt (CTRL-C signal)
	signal.Notify(osSignals, os.Interrupt)

	select {
	case osSignal := <-osSignals:
		if osSignal == os.Interrupt {
			evPS.Pub(true, OsExit)
		}
	}
}
