package audio

import (
	"fmt"

	"github.com/gordonklaus/portaudio"
)

func adjustChannels(iChs, oChs int, audioFrames []float32) []float32 {
	// mono -> stereo
	if iChs == 1 && oChs == 2 {
		res := make([]float32, 0, len(audioFrames)*2)
		// left channel = right channel
		for _, frame := range audioFrames {
			res = append(res, frame)
			res = append(res, frame)
		}
		return res
	}

	// stereo -> mono
	res := make([]float32, 0, len(audioFrames)/2)
	// chop off the right channel
	for i := 0; i < len(audioFrames); i += 2 {
		res = append(res, audioFrames[i])
	}
	return res
}

func adjustVolume(volume float32, audioFrames []float32) {
	for i := 0; i < len(audioFrames); i++ {
		audioFrames[i] *= volume
	}
}

// getPaDevice checks if the Audio Devices actually exist and
// then returns it
func getPaDevice(name string) (*portaudio.DeviceInfo, error) {
	devices, _ := portaudio.Devices()
	for _, device := range devices {
		if device.Name == name {
			return device, nil
		}
	}
	return nil, fmt.Errorf("unknown audio device %s", name)
}
