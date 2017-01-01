package audio

import (
	"fmt"
	"time"

	"github.com/dh1tw/gosamplerate"
	"github.com/dh1tw/opus"
	"github.com/gordonklaus/portaudio"
	"github.com/spf13/viper"
	ringBuffer "github.com/zfjagann/golang-ring"
)

//PlayerSync plays received audio on a local audio device
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
			fmt.Println("unable to find default playback sound device")
			fmt.Println(err)
			return // exit go routine
		}
	} else {
		if err := ad.IdentifyDevice(); err != nil {
			fmt.Printf("unable to find recording sound device %s\n", ad.DeviceName)
			fmt.Println(err)
			return
		}
	}

	ad.out = make([]float32, 500000)

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
		fmt.Printf("unable to open playback audio stream on device %s\n", ad.DeviceName)
		fmt.Println(err)
		return // exit go routine
	}
	defer stream.Close()

	ad.Converter, err = gosamplerate.New(viper.GetInt("output_device.quality"), ad.Channels, 65536)
	if err != nil {
		fmt.Println("unable to create resampler")
		fmt.Println(err)
		return // exit go routine
	}
	defer gosamplerate.Delete(ad.Converter)

	if err = stream.Start(); err != nil {
		fmt.Printf("unable to start playback audio stream on device %s\n", ad.DeviceName)
		fmt.Println(err)
		return // exit go routine
	}

	defer stream.Stop()
	// enableLoopback := false

	var d deserializer
	d.AudioDevice = &ad

	opusDecoder, err := opus.NewDecoder(int(ad.Samplingrate), ad.Channels)
	fmt.Println("opus decoder channels:", ad.Channels)
	// opusDecoder, err := opus.NewDecoder(int(d.Samplingrate), 2)
	if err != nil || opusDecoder == nil {
		fmt.Println(err)
		return
	}
	d.opusDecoder = opusDecoder

	d.opusBuffer = make([]float32, 100000)

	r := ringBuffer.Ring{}
	r.SetCapacity(10)

	for {
		select {
		case <-ad.EventCh:
			// TBD
		case msg := <-ad.ToDeserialize:
			r.Enqueue(msg.Data)
		default:
			data := r.Dequeue()
			if data != nil {
				err := d.DeserializeAudioMsg(data.([]byte))
				if err != nil {
					fmt.Println(err)
				} else {
					stream.Write()
				}
			} else {
				time.Sleep(time.Microsecond * 100)
			}
		}
	}
}
