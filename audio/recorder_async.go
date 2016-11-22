package audio

import (
	"fmt"
	"os"

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

	stream, err = portaudio.OpenStream(streamParm, ad.recordCb)

	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	defer stream.Stop()

	ad.Converter, err = samplerate.New(samplerate.SRC_LINEAR, ad.Channels, 65536)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	defer samplerate.Delete(ad.Converter)

	stream.Start()
	defer stream.Close()

	for {
		select {
		case msg := <-ad.ToSerialize:
			// serialize the Audio data and send to for
			// transmission to the comms coroutine
			data, err := ad.SerializeAudioMsg(msg.Raw)
			if err != nil {
				fmt.Println(err)
			} else {
				msg := AudioMsg{}
				msg.Topic = viper.GetString("mqtt.topic_audio_out")
				msg.Data = data
				ad.ToWire <- msg
			}
		}
	}
}

func (ad *AudioDevice) recordCb(in []float32, iTime portaudio.StreamCallbackTimeInfo, iFlags portaudio.StreamCallbackFlags) {
	switch iFlags {
	case portaudio.InputUnderflow:
		fmt.Println("InputUnderflow")
	case portaudio.InputOverflow:
		fmt.Println("InputOverflow")
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
