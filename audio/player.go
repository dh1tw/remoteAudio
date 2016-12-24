package audio

import (
	"fmt"
	"os"

	samplerate "github.com/dh1tw/samplerate"
	"github.com/gordonklaus/portaudio"
)

func PlayerSync(ad AudioDevice) {

	portaudio.Initialize()
	defer portaudio.Terminate()

	//out doesn't need to be initialized with a fixed buffer size
	//since the slice will be copied from the incoming data
	//and will therefore replay any buffer size

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

	stream, err = portaudio.OpenStream(streamParm, &ad.out)
	if err != nil {
		fmt.Println(err)
	}
	defer stream.Close()

	ad.Converter, err = samplerate.New(samplerate.SRC_LINEAR, ad.Channels, 65536)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	defer samplerate.Delete(ad.Converter)

	defer stream.Stop()

	stream.Start()

	// enableLoopback := false

	for {
		select {
		case msg := <-ad.ToDeserialize:
			// if !enableLoopback {
			// fmt.Println(stream.Info())
			err := ad.DeserializeAudioMsg(msg.Data)
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
