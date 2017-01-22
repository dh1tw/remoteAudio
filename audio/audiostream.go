package audio

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cskr/pubsub"
	"github.com/dh1tw/gosamplerate"
	"github.com/dh1tw/remoteAudio/comms"
	sbAudio "github.com/dh1tw/remoteAudio/sb_audio"
	"github.com/gordonklaus/portaudio"
	"gopkg.in/hraban/opus.v2"
)

const (
	INPUT  = 1
	OUTPUT = 2
)

const (
	MONO   = 1
	STEREO = 2
)

const (
	PCM  = 1
	OPUS = 2
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
	DeviceName             string
	Direction              int
	Channels               int
	Samplingrate           float64
	Latency                time.Duration
	FramesPerBuffer        int
	Device                 *portaudio.DeviceInfo
	Stream                 *portaudio.Stream
	PCMSamplerateConverter gosamplerate.Src
	out                    []float32
	in                     []float32
}

// AudioDevice contains the configuration for an Audio Device
type AudioDevice struct {
	AudioStream
	ToSerialize      chan comms.IOMsg
	ToWire           chan comms.IOMsg
	ToDeserialize    chan []byte
	AudioToWireTopic string
	Events           *pubsub.PubSub
	WaitGroup        *sync.WaitGroup
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

// GetOpusApplication returns the integer representation of a
// Opus application value string (typically read from application settings)
func GetOpusApplication(app string) (opus.Application, error) {
	switch app {
	case "audio":
		return opus.AppAudio, nil
	case "restricted_lowdelay":
		return opus.AppRestrictedLowdelay, nil
	case "voip":
		return opus.AppVoIP, nil
	}
	return 0, errors.New("unknown opus application value")
}

// GetOpusMaxBandwith returns the integer representation of an
// Opus max bandwitdh value string (typically read from application settings)
func GetOpusMaxBandwith(maxBw string) (opus.Bandwidth, error) {
	switch strings.ToLower(maxBw) {
	case "narrowband":
		return opus.Narrowband, nil
	case "mediumband":
		return opus.Mediumband, nil
	case "wideband":
		return opus.Wideband, nil
	case "superwideband":
		return opus.SuperWideband, nil
	case "fullband":
		return opus.Fullband, nil
	}

	return 0, errors.New("unknown opus max bandwidth value")
}

// GetCodec return the integer representation of a string containing the
// name of an audio codec
func GetCodec(codec string) (int, error) {
	switch strings.ToLower(codec) {
	case "pcm":
		return PCM, nil
	case "opus":
		return OPUS, nil
	}
	errMsg := fmt.Sprintf("unknown codec: %s", codec)
	return 0, errors.New(errMsg)
}

// Sync Pool for Protocol Buffers Audio objects (to reduce memory allocation / garbage collection for short lived objects)
var sbAudioDataPool = sync.Pool{
	New: func() interface{} {
		return &sbAudio.AudioData{}
	},
}
