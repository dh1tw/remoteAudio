package audio

import (
	"fmt"
	"os"

	"github.com/gordonklaus/portaudio"
)

type AudioMsg struct {
	Data  []byte
	Raw   []int16
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

	//out doesn't need to be initialized with a fixed buffer size
	//since the slice will be copied from the incoming data
	//and will therefore replay any buffer size
	// var out []int16

	ad.out.Data32 = make([]int32, ad.FramesPerBuffer*ad.Channels)
	ad.out.Data16 = make([]int16, ad.FramesPerBuffer*ad.Channels)
	ad.out.Data8 = make([]int8, ad.FramesPerBuffer*ad.Channels)

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

	var stream *portaudio.Stream

	if ad.Bitrate == 8 {
		stream, err = portaudio.OpenStream(streamParm, ad.out.Data8)
	} else if ad.Bitrate == 16 {
		stream, err = portaudio.OpenStream(streamParm, ad.out.Data16)
	} else if ad.Bitrate == 32 {
		stream, err = portaudio.OpenStream(streamParm, ad.out.Data32)
	}

	if err != nil {
		fmt.Println(err)
	}

	defer stream.Close()
	defer stream.Stop()

	stream.Start()

	// enableLoopback := false

	for {
		select {
		case msg := <-ad.AudioInCh:
			// if !enableLoopback {
			// fmt.Println(stream.Info())
			err := ad.deserializeAudioMsg(msg.Data)
			if err != nil {
				fmt.Println(err)
			} else {
				stream.Write()
			}
			// }
			// case echo := <-ad.AudioLoopbackCh:
			// 	// fmt.Println(stream.Info())
			// 	out = echo.Raw
			// 	stream.Write()

			// case ev := <-ad.EventCh:
			// 	enableLoopback = ev.(events.Event).EnableLoopback
		}
	}
}
