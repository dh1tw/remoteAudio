package audio

import (
	"fmt"
	"time"

	"github.com/dh1tw/gosamplerate"
	"github.com/gordonklaus/portaudio"
	"github.com/spf13/viper"
)

// RecorderSync records synchronously Audio from a AudioDevice
func RecorderSync(ad AudioDevice) {

	portaudio.Initialize()
	defer portaudio.Terminate()

	var deviceInfo *portaudio.DeviceInfo
	var err error

	ad.in = make([]float32, ad.FramesPerBuffer*ad.Channels)

	if ad.DeviceName == "default" {
		deviceInfo, err = portaudio.DefaultInputDevice()
		if err != nil {
			fmt.Println("unable to find default recording sound device")
			fmt.Println(err)
			return // exit go routine
		}
	} else {
		if err := ad.IdentifyDevice(); err != nil {
			fmt.Printf("unable to find recording sound device %s\n", ad.DeviceName)
			fmt.Println(err)
			return // exit go routine
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

	stream, err = portaudio.OpenStream(streamParm, &ad.in)

	if err != nil {
		fmt.Printf("unable to open recording audio stream on device %s\n", ad.DeviceName)
		fmt.Println(err)
		return // exit go routine
	}

	defer stream.Stop()

	ad.Converter, err = gosamplerate.New(viper.GetInt("input_device.quality"), ad.Channels, 65536)
	//	ad.Converter, err = gosamplerate.New(gosamplerate.SRC_SINC_MEDIUM_QUALITY, ad.Channels, 65536)
	if err != nil {
		fmt.Println("unable to create resampler")
		fmt.Println(err)
		return // exit go routine
	}
	defer gosamplerate.Delete(ad.Converter)

	if err = stream.Start(); err != nil {
		fmt.Printf("unable to start recording audio stream on device %s\n", ad.DeviceName)
		fmt.Println(err)
		return // exit go routine
	}
	defer stream.Close()

	var s serializer
	s.AudioDevice = &ad
	s.wireSamplingrate = viper.GetFloat64("wire.samplingrate")
	s.wireOutputChannels = GetChannel(viper.GetString("wire.output_channels"))
	s.framesPerBufferI = int32(ad.FramesPerBuffer)
	s.samplingRateI = int32(s.wireSamplingrate)
	s.channelsI = int32(s.wireOutputChannels)
	s.bitrateI = int32(viper.GetInt("wire.bitrate"))
	s.userID = string(viper.GetString("user.user_id"))

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
			data, err := s.SerializeAudioMsg(s.in)
			if err != nil {
				fmt.Println(err)
			} else {
				msg := AudioMsg{}
				msg.Topic = viper.GetString("mqtt.topic_audio_out")
				msg.Data = data
				ad.ToWire <- msg
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
