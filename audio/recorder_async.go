package audio

import (
	"fmt"

	"github.com/dh1tw/gosamplerate"
	"github.com/dh1tw/opus"
	"github.com/dh1tw/remoteAudio/events"
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
	s.pcmSamplingrate = int32(viper.GetFloat64("pcm.samplingrate"))
	s.pcmBufferSize = int32(ad.FramesPerBuffer)
	s.pcmChannels = int32(GetChannel(viper.GetString("pcm.channels")))
	s.pcmBitDepth = int32(viper.GetInt("pcm.bitdepth"))

	app, err := GetOpusApplication(viper.GetString("opus.application"))
	if err != nil {
		fmt.Println(err)
		return
	}

	// initialize Opus Encoder

	opusEncoder, err := opus.NewEncoder(int(ad.Samplingrate), ad.Channels,
		app)
	if err != nil || opusEncoder == nil {
		fmt.Println(err)
		return
	}

	err = opusEncoder.SetBitrate(viper.GetInt("opus.bitrate"))
	if err != nil {
		fmt.Println("invalid Opus bitrate", err)
		return
	}

	err = opusEncoder.SetComplexity(viper.GetInt("opus.complexity"))
	if err != nil {
		fmt.Println("invalid Opus complexity value", err)
		return
	}

	maxBw, err := GetOpusMaxBandwith(viper.GetString("opus.max_bandwidth"))
	if err != nil {
		fmt.Println(err)
		return
	}

	err = opusEncoder.SetMaxBandwidth(maxBw)
	if err != nil {
		fmt.Println(err)
		return
	}

	s.opusEncoder = opusEncoder
	s.opusBuffer = make([]byte, 520000)

	stream, err = portaudio.OpenStream(streamParm, s.recordCb)

	if err != nil {
		fmt.Printf("unable to open recording audio stream on device %s\n", ad.DeviceName)
		fmt.Println(err)
		return // exit go routine
	}

	defer stream.Stop()

	ad.Converter, err = gosamplerate.New(viper.GetInt("input_device.quality"), ad.Channels, 65536)
	if err != nil {
		fmt.Println("unable to create resampler")
		fmt.Println(err)
		return // exit go routine
	}
	defer gosamplerate.Delete(ad.Converter)

	// if err = stream.Start(); err != nil {
	// 	fmt.Printf("unable to start recording audio stream on device %s\n", ad.DeviceName)
	// 	fmt.Println(err)
	// 	return // exit go routine
	// }

	// defer stream.Close()

	codec, err := GetCodec(viper.GetString("audio.codec"))
	if err != nil {
		fmt.Println(err)
		return
	}

	mqttTopicAudioOut := viper.GetString("mqtt.topic_audio_out")

	for {
		select {
		case msg := <-ad.EventCh:
			ev := msg.(events.Event)
			if ev.SendAudio {
				stream.Start()
			} else if !ev.SendAudio {
				stream.Stop()
			}
		case msg := <-ad.ToSerialize:
			// if ptt {
			// serialize the Audio data and send to for
			// transmission to the comms coroutine
			var data []byte
			var err error
			if codec == OPUS {
				data, err = s.SerializeOpusAudioMsg(msg.Raw)
			} else if codec == PCM {
				data, err = s.SerializePCMAudioMsg(msg.Raw)
			}
			if err != nil {
				fmt.Println(err)
			} else {
				msg := AudioMsg{}
				msg.Topic = mqttTopicAudioOut
				msg.Data = data
				ad.ToWire <- msg
			}
			// }
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
