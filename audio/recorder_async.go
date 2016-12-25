package audio

import (
	"fmt"

	"github.com/dh1tw/samplerate"
	"github.com/gordonklaus/portaudio"
	"github.com/spf13/viper"
)

// RecorderAsync grabs audio asynchronously from an AudioDevice
func RecorderAsync(ad AudioDevice) {

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
			return //exit go routine
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

	var s serializer
	s.AudioDevice = &ad
	s.wireSamplingrate = viper.GetFloat64("wire.samplingrate")
	s.wireOutputChannels = GetChannel(viper.GetString("wire.output_channels"))
	s.framesPerBufferI = int32(ad.FramesPerBuffer)
	s.samplingRateI = int32(s.wireSamplingrate)
	s.channelsI = int32(s.wireOutputChannels)
	s.bitrateI = int32(viper.GetInt("wire.bitrate"))

	stream, err = portaudio.OpenStream(streamParm, s.recordCb)

	if err != nil {
		fmt.Printf("unable to open recording audio stream on device %s\n", ad.DeviceName)
		fmt.Println(err)
		return // exit go routine
	}

	defer stream.Stop()

	ad.Converter, err = samplerate.New(viper.GetInt("input_device.quality"), ad.Channels, 65536)
	if err != nil {
		fmt.Println("unable to create resampler")
		fmt.Println(err)
		return // exit go routine
	}
	defer samplerate.Delete(ad.Converter)

	if err = stream.Start(); err != nil {
		fmt.Printf("unable to start recording audio stream on device %s\n", ad.DeviceName)
		fmt.Println(err)
		return // exit go routine
	}

	defer stream.Close()

	mqttTopicAudioOut := viper.GetString("mqtt.topic_audio_out")

	for {
		select {
		case msg := <-ad.ToSerialize:
			// serialize the Audio data and send to for
			// transmission to the comms coroutine
			data, err := s.SerializeAudioMsg(msg.Raw)
			if err != nil {
				fmt.Println(err)
			} else {
				msg := AudioMsg{}
				msg.Topic = mqttTopicAudioOut
				msg.Data = data
				ad.ToWire <- msg
			}
		}
	}
}

func (ad *AudioDevice) recordCb(in []float32, iTime portaudio.StreamCallbackTimeInfo, iFlags portaudio.StreamCallbackFlags) {
	switch iFlags {
	case portaudio.InputOverflow:
		fmt.Println("InputOverflow")
		return // data was lost (to be confirmed)
	}
	// a deep copy is necessary, since portaudio reuses the slice "in"
	buf := make([]float32, len(in))
	for i, v := range in {
		buf[i] = v
	}
	// keep the callback as short as possible
	// sent to raw data to another coroutine for serialization
	msg := AudioMsg{}
	msg.Raw = buf
	ad.ToSerialize <- msg
}
