package audio

import (
	"fmt"
	"os"
	"time"

	"github.com/gordonklaus/portaudio"
	"github.com/spf13/viper"
)

func RecorderSync(ad AudioDevice) {

	portaudio.Initialize()
	defer portaudio.Terminate()

	ad.in.Data32 = make([]float32, ad.FramesPerBuffer*ad.Channels)

	var deviceInfo *portaudio.DeviceInfo
	var err error

	if ad.DeviceName == "default" {
		deviceInfo, err = portaudio.DefaultInputDevice()
		if err != nil {
			fmt.Println(err)
		}
	} else {
		if err := ad.IdentifyDevice(); err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
	}

	// streamParm := portaudio.LowLatencyParameters(deviceInfo, nil)

	streamDeviceParam := portaudio.StreamDeviceParameters{
		Device:   deviceInfo,
		Channels: ad.Channels,
		Latency:  ad.Latency,
	}

	streamParm := portaudio.StreamParameters{
		FramesPerBuffer: ad.FramesPerBuffer,
		Input:           streamDeviceParam,
		SampleRate:      ad.Samplingrate,
	}

	var stream *portaudio.Stream

	stream, err = portaudio.OpenStream(streamParm, &ad.in.Data32)

	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	defer stream.Close()
	defer stream.Stop()

	stream.Start()

	for {
		num, err := stream.AvailableToRead()

		if err != nil {
			fmt.Println(err)
		}

		if num > 0 {
			err := stream.Read()
			if err != nil {
				fmt.Println(err)
			}
			data, err := ad.serializeAudioMsg()
			if err != nil {
				fmt.Println(err)
			} else {
				msg := AudioMsg{}
				msg.Topic = viper.GetString("mqtt.topic_audio_out")
				msg.Data = data
				ad.AudioOutCh <- msg
			}
		}
		select {
		// case ev := <-ad.EventCh:
		// 	enableLoopback = ev.(events.Event).EnableLoopback
		// 	fmt.Println("Loopback (Recorder):", enableLoopback)
		default:
			time.Sleep(time.Microsecond * 200)
		}
	}
}
