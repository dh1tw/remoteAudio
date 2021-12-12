package doorman

import (
	"log"
	"sync"
	"time"

	"github.com/dh1tw/remoteAudio/audio"
)

// Doorman is the datastructure that holds the variables for the audio.Node.
type Doorman struct {
	sync.Mutex
	lastUser        string
	lastHeard       time.Time
	onDataCb        audio.OnDataCb
	onTxUserChanged func(string)
}

// NewDoorman returns an instance of a Doorman audio Node. It implements the
// audio.Node interface. It's purpose is to avoid two clients transmitting
// audio at the same time. Through a functional option an callback can be
// provided which will be called whenever the active (transmitting) client
// changes.
func NewDoorman(opts ...Option) (*Doorman, error) {

	d := &Doorman{
		lastUser:  "",
		lastHeard: time.Now(),
	}

	options := Options{}

	for _, option := range opts {
		option(&options)
	}

	if options.txUserChangedCb != nil {
		d.onTxUserChanged = options.txUserChangedCb
	}

	// this go-routine checks periodically if audio messages are still
	// received from a particular client. If not, the Lock (doorman.lastUser)
	// will be cleared.
	go func() {
		inUseTicker := time.NewTicker(time.Millisecond * 100)
		for {
			<-inUseTicker.C
			d.Lock()
			if time.Since(d.lastHeard) > time.Duration(time.Millisecond*200) {
				if d.lastUser != "" {
					d.lastUser = ""
					// inform the application that the txUser has been cleared
					// in case the callback is set
					if d.onTxUserChanged != nil {
						go d.onTxUserChanged("")
					}
				}
			}
			d.Unlock()
		}
	}()

	return d, nil
}

// Write is the entry point into this audio Node. Writing an audio.Msg
// will start the processing.
func (d *Doorman) Write(msg audio.Msg) error {

	// make sure the Metadata dict exists
	if msg.Metadata == nil {
		return nil
	}

	// make sure the userID key has been set
	userID, ok := msg.Metadata["userID"]
	if !ok {
		return nil
	}

	txUser := ""

	// make sure the interface{} can be casted to a string
	switch uID := userID.(type) {
	default:
		log.Println("doorman: can not cast userID to string")
		return nil
	case string:
		txUser = uID
	}

	d.Lock()
	lastUser := d.lastUser
	lastHeard := d.lastHeard
	d.Unlock()

	// in case this is the current txUser forward the msg
	if lastUser == txUser {
		d.Lock()
		d.lastHeard = time.Now() //update the timestamp
		if d.onDataCb != nil {
			// call callback asynchronously to pass the data to the next node
			go d.onDataCb(msg)
		}
		d.Unlock()
		return nil
	}

	// in case d.lastUser != txUser, but we don't expect any more audio msgs
	// from the original txUser
	if time.Since(lastHeard) >= time.Duration(time.Millisecond*100) {
		d.Lock()
		d.lastUser = txUser
		d.lastHeard = time.Now()
		if d.onDataCb != nil {
			// call callback asynchronously to pass the data to the next node
			go d.onDataCb(msg)
		}
		if d.onTxUserChanged != nil {
			// notify application that txUser has changed.
			go d.onTxUserChanged(txUser)
		}
		d.Unlock()
	}

	// if d.lastUser != txUser and d.lastUser heard within the last
	// 100ms, we drop the msg

	return nil
}

// SetCb sets the callback which will be called when the data has been
// processed and is ready to be sent to the next audio.Node or audio.Sink.
func (d *Doorman) SetCb(cb audio.OnDataCb) {
	d.Lock()
	defer d.Unlock()
	d.onDataCb = cb
}
