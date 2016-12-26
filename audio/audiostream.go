package audio

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/dh1tw/gosamplerate"
	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
	"github.com/gordonklaus/portaudio"
)

const (
	INPUT  = 1
	OUTPUT = 2
)

const (
	MONO   = 1
	STEREO = 2
)

var bitMapToInt32 = map[int32]float32{
	8:  255,
	12: 4095,
	16: 32767,
	32: 2147483647,
}

var bitMapToFloat32 = map[int]float32{
	8:  256,
	12: 4096,
	16: 32768,
	32: 2147483648,
}

// AudioStream contains the configuration for a portaudio Audiostream
type AudioStream struct {
	DeviceName      string
	Direction       int
	Channels        int
	Samplingrate    float64
	Latency         time.Duration
	FramesPerBuffer int
	Device          *portaudio.DeviceInfo
	Stream          *portaudio.Stream
	Converter       gosamplerate.Src
	out             []float32
	in              []float32
}

// AudioMsg is a struct for internal communication
type AudioMsg struct {
	Data  []byte
	Raw   []float32
	Topic string
}

// AudioDevice contains the configuration for an Audio Device
type AudioDevice struct {
	AudioStream
	ToSerialize     chan AudioMsg
	ToWire          chan AudioMsg
	ToDeserialize   chan AudioMsg
	AudioLoopbackCh chan AudioMsg
	EventCh         chan interface{}
}

// IdentifyDevice checks if the Audio Devices actually exist
func (as *AudioDevice) IdentifyDevice() error {
	devices, _ := portaudio.Devices()
	for _, device := range devices {
		if device.Name == as.DeviceName {
			as.Device = device
			return nil
		}
	}
	return errors.New("unknown audio device")
}

// GetChannel returns the integer representation of channels
func GetChannel(ch string) int {
	if strings.ToUpper(ch) == "MONO" {
		return MONO
	} else if strings.ToUpper(ch) == "STEREO" {
		return STEREO
	}
	return 0
}

// Sync Pool for Protocol Buffers Audio objects (to reduce memory allocation / garbage collection for short lived objects)
var sbAudioDataPool = sync.Pool{
	New: func() interface{} {
		return &sbAudio.AudioData{}
	},
}
