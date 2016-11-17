package audio

import (
	"errors"
	"strings"
	"time"

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

type AudioStream struct {
	DeviceName      string
	Direction       int
	Channels        int
	Samplingrate    float64
	Bitrate         int
	Latency         time.Duration
	FramesPerBuffer int
	Device          *portaudio.DeviceInfo
	Stream          *portaudio.Stream
	out             AudioData
	in              AudioData
}

type AudioSamples struct {
	Channels     uint32
	Samplingrate uint32
	Frames       uint32
	Bitrate      uint32
	Data8        []int8
	Data16       []int16
}

type AudioData struct {
	Data32 []float32
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

func GetChannel(ch string) int {
	if strings.ToUpper(ch) == "MONO" {
		return MONO
	} else if strings.ToUpper(ch) == "STEREO" {
		return STEREO
	}
	return 0
}
