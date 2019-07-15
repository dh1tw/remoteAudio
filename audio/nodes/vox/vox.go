package vox

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/chewxy/math32"

	"github.com/dh1tw/remoteAudio/audio"
)

// Vox is an Audio Node which detects if the audio level raises above or falls
// below a defined threshold level.
type Vox struct {
	sync.Mutex
	enabled        bool
	active         bool
	lastActivation time.Time
	cb             audio.OnDataCb
	onStateChange  func(voxOn bool)
	threshold      float32
	holdTime       time.Duration
	chWarning      sync.Once
}

// New is the constructor method for a Vox Object. Vox implements
// an audio.Node and emits a StateChanged callback when the RMS
// (root mean square) has fallen above or below the set threshold. By
// default the threshold is set to 0.1 and the hold time to 500ms.
func New(opts ...Option) *Vox {
	v := &Vox{
		cb:             nil,
		holdTime:       time.Millisecond * 500,
		threshold:      0.1,
		lastActivation: time.Time{},
	}

	for _, opt := range opts {
		opt(v)
	}

	return v
}

// Write is the entry point into this audio Node. Writing an audio.Msg
// will start the processing.
func (v *Vox) Write(msg audio.Msg) error {
	v.Lock()
	defer v.Unlock()

	if v.cb == nil {
		return nil
	}

	// forward the msg asap to the next node
	go v.cb(msg)

	if !v.enabled {
		return nil
	}

	if msg.Channels > 1 {
		v.multiChannelWarning()
	}

	// empty frame
	if len(msg.Data) == 0 {
		return nil
	}

	rmsValue, err := rms(msg.Data)
	if err != nil {
		return err
	}

	if rmsValue >= v.threshold {
		v.lastActivation = time.Now()
		if !v.active {
			v.active = true
			// log.Println("activating vox")
			go v.onStateChange(true)
		}
	} else {
		if v.active && time.Since(v.lastActivation) > v.holdTime {
			v.active = false
			// log.Println("deactivating vox")
			go v.onStateChange(false)
		}
	}

	return nil
}

// SetCb sets the callback which will be called when the data has been
// processed and is ready to be sent to the next audio.Node or audio.Sink.
func (v *Vox) SetCb(cb audio.OnDataCb) {
	v.Lock()
	defer v.Unlock()
	v.cb = cb
}

// Enable or disable the vox. If the vox is disabled, the audio data
// will be passed on to the next audio node in the chain.
func (v *Vox) Enable(state bool) {
	v.Lock()
	defer v.Unlock()
	v.enabled = state
}

// Enabled returns a boolean value indicating if the vox is
// enabled. If not, the audio data is directly passed on to the
// next audio node in the chain.
func (v *Vox) Enabled() bool {
	v.Lock()
	defer v.Unlock()
	return v.enabled
}

// SetThreshold sets the value for the vox threshold. Only values
// between 0...1 are allowed. Values below or above will be clipped
// to the minimum or maximum.
func (v *Vox) SetThreshold(value float32) {
	v.Lock()
	defer v.Unlock()
	if value > 1.0 {
		v.threshold = 1.0
	} else if value < 0.0 {
		v.threshold = 0.0
	} else {
		v.threshold = value
	}
}

// Threshold returns the vox threshold value.
func (v *Vox) Threshold() float32 {
	v.Lock()
	defer v.Unlock()
	return v.threshold
}

// SetHoldTime sets the vox hold time. The hold time is the duration
// which will be waited until a statechange event is emitted.
func (v *Vox) SetHoldTime(t time.Duration) {
	v.Lock()
	defer v.Unlock()
	v.holdTime = t
}

// Holdtime returns the current vox holdtime
func (v *Vox) Holdtime() time.Duration {
	v.Lock()
	defer v.Unlock()
	return v.holdTime
}

// calculate the root mean square for a non-interlaced audio
// frame
func rms(data []float32) (float32, error) {

	var sum float32

	if len(data) == 0 {
		return sum, fmt.Errorf("empty slice provided")
	}

	for _, el := range data {
		sum = sum + el*el
	}

	sum = sum / float32(len(data))

	return math32.Sqrt(sum), nil
}

func (v *Vox) multiChannelWarning() {
	v.chWarning.Do(func() {
		log.Println("WARNING: multiple input channels detected; RMS for Vox will be calculated over all channel samples")
	})
}
