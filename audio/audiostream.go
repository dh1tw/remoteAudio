package audio

import (
	"errors"
	"fmt"
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
	Latency         time.Duration
	FramesPerBuffer int
	Device          *portaudio.DeviceInfo
	Stream          *portaudio.Stream
	out             AudioData
	in              AudioData
}

type AudioSamples struct {
	Channels     int32
	Samplingrate int32
	Frames       int32
	Data         []int32
}

// Audiodata is an Interface to handle either incoming or outgoing audio data
type AudioData interface {
	// Process(StereoSamples)
	Process([]int32)
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
	fmt.Println("ch")
	if strings.ToUpper(ch) == "MONO" {
		fmt.Println("ch")
		return MONO
	} else if strings.ToUpper(ch) == "STEREO" {
		fmt.Println("ch")
		return STEREO
	}

	return 0
}
