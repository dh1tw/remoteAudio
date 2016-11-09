package audio

import (
	"fmt"
	"os"

	"github.com/dh1tw/remoteAudio/events"
	"github.com/gordonklaus/portaudio"
)

type AudioMsg struct {
	Data  []byte
	Raw   []int32
	Topic string
}

type AudioDevice struct {
	AudioStream
	AudioInCh       chan AudioMsg
	AudioOutCh      chan AudioMsg
	AudioLoopbackCh chan AudioMsg
	EventCh         chan interface{}
}

func PlayerSync(ad AudioDevice) {

	portaudio.Initialize()
	defer portaudio.Terminate()

	out := make([]int32, ad.FramesPerBuffer)

	var deviceInfo *portaudio.DeviceInfo
	var err error

	if ad.DeviceName == "default" {
		deviceInfo, err = portaudio.DefaultOutputDevice()
		if err != nil {
			fmt.Println(err)
		}
	} else {
		if err := ad.IdentifyDevice(); err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
	}

	// streamParm := portaudio.LowLatencyParameters(nil, deviceInfo)

	streamDeviceParam := portaudio.StreamDeviceParameters{
		Device:   deviceInfo,
		Channels: ad.Channels,
		Latency:  ad.Latency,
	}

	streamParm := portaudio.StreamParameters{
		FramesPerBuffer: ad.FramesPerBuffer,
		Output:          streamDeviceParam,
		SampleRate:      ad.Samplingrate,
	}

	stream, err := portaudio.OpenStream(
		streamParm,
		&out)

	if err != nil {
		fmt.Println(err)
	}

	defer stream.Close()
	defer stream.Stop()

	stream.Start()

	enableLoopback := false

	for {
		select {
		case msg := <-ad.AudioInCh:
			if !enableLoopback {
				// fmt.Println(stream.Info())
				data, err := deserializeAudioMsg(msg.Data)
				if err != nil {
					fmt.Println(err)
				} else {
					out = data.Data
					stream.Write()
				}
			}
		case echo := <-ad.AudioLoopbackCh:
			// fmt.Println(stream.Info())
			out = echo.Raw
			stream.Write()

		case ev := <-ad.EventCh:
			enableLoopback = ev.(events.Event).EnableLoopback
		}
	}
}
