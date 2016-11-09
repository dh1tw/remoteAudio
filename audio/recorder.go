package audio

import (
	"fmt"
	"os"
	"time"

	"github.com/dh1tw/remoteAudio/events"
	"github.com/gordonklaus/portaudio"
	"github.com/spf13/viper"
)

func RecorderSync(ad AudioDevice) {

	portaudio.Initialize()
	defer portaudio.Terminate()

	in := make([]int32, ad.FramesPerBuffer)

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
		Latency:  time.Millisecond * 5,
	}

	streamParm := portaudio.StreamParameters{
		FramesPerBuffer: ad.FramesPerBuffer,
		Input:           streamDeviceParam,
		SampleRate:      ad.Samplingrate,
	}

	stream, err := portaudio.OpenStream(
		streamParm,
		&in)

	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	defer stream.Close()
	defer stream.Stop()

	stream.Start()

	enableLoopback := false

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

			data, err := ad.serializeAudioMsg(in)
			if err != nil {
				fmt.Println(err)
			} else {
				msg := AudioMsg{}
				msg.Topic = viper.GetString("mqtt.topic_audio_out")
				msg.Data = data
				ad.AudioOutCh <- msg
			}

			if enableLoopback {
				buf := make([]int32, 0, len(in))
				for _, sample := range in {
					buf = append(buf, sample)
				}
				msg := AudioMsg{}
				msg.Raw = buf
				ad.AudioLoopbackCh <- msg
			}
		}
		select {
		case ev := <-ad.EventCh:
			enableLoopback = ev.(events.Event).EnableLoopback
			fmt.Println("Loopback (Recorder):", enableLoopback)
		default:
			time.Sleep(time.Millisecond * 1)
		}
	}
}
