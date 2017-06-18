package audio

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dh1tw/gosamplerate"
	"github.com/dh1tw/remoteAudio/comms"
	"github.com/dh1tw/remoteAudio/events"
	"github.com/gordonklaus/portaudio"
	"github.com/spf13/viper"
	"gopkg.in/hraban/opus.v2"
)

// RecorderAsync grabs audio asynchronously from an a local audio device
func RecorderAsync(ad AudioDevice) {

	defer ad.WaitGroup.Done()

	// Subscribe on events
	recordAudioCh := ad.Events.Sub(events.RecordAudioOn)
	shutdownCh := ad.Events.Sub(events.Shutdown)

	// Initialize Portaudio
	portaudio.Initialize()
	defer portaudio.Terminate()

	// give Portaudio a few milliseconds to initialize
	// this is necessary to avoid a SIGSEGV in case
	// DefaultInputDevice is accessed without portaudio
	// being completely initialized
	time.Sleep(time.Millisecond * 300)

	var deviceInfo *portaudio.DeviceInfo
	var err error

	// initialize Audio Buffer
	ad.in = make([]float32, ad.FramesPerBuffer*ad.Channels)

	// select Recording Audio Device
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

	// setup Audio Stream
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

	// the serializer struct is mainly used to cache variables which are
	// frequently written into a protocol buffers message
	// viper Lookups are unfortunately CPU intensive

	userID := viper.GetString("mqtt.client-id")

	var s serializer
	s.AudioDevice = &ad
	s.userID = userID
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

	maxBw, err := GetOpusMaxBandwith(viper.GetString("opus.max-bandwidth"))
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
	s.opusBuffer = make([]byte, 520000) //max opus message size
	s.opusChannels = int32(ad.Channels)

	// open the audio stream
	stream, err = portaudio.OpenStream(streamParm, s.recordCb)

	if err != nil {
		fmt.Printf("unable to open recording audio stream on device %s\n", ad.DeviceName)
		fmt.Println(err)
		return // exit go routine
	}
	defer stream.Stop()

	// create the PCM samplerate converter
	ad.PCMSamplerateConverter, err = gosamplerate.New(viper.GetInt("input-device.quality"), ad.Channels, 65536)
	if err != nil {
		fmt.Println("unable to create PCM samplerate converter")
		fmt.Println(err)
		return // exit go routine
	}
	defer gosamplerate.Delete(ad.PCMSamplerateConverter)

	codec, err := GetCodec(viper.GetString("audio.codec"))
	if err != nil {
		fmt.Println(err)
		return
	}

	err = stream.Start()
	if err != nil {
		fmt.Println(err)
	}

	muSend := sync.RWMutex{}
	sendAudio := false

	// Everything has been set up, let's start exection

	for {
		select {

		// shutdown application gracefully
		case <-shutdownCh:
			log.Println("Shutdown Recorder")
			stream.Stop()
			return

		// start or stop the Audio recording
		case msg := <-recordAudioCh:
			rxAudioOn := msg.(bool)
			muSend.Lock()
			if rxAudioOn {
				// err = stream.Start()
				sendAudio = true
				log.Println("starting audio stream")
			} else {
				// err = stream.Stop()
				sendAudio = false
				log.Println("stopping audio stream")
			}
			muSend.Unlock()
			if err != nil {
				fmt.Println(err)
			}

		// Serialize the Audio Data (PCM or OPUS)
		case msg := <-ad.ToSerialize:
			muSend.RLock()
			ms := sendAudio
			muSend.RUnlock()
			if ms {
				var data []byte
				var err error
				if codec == OPUS {
					data, err = s.SerializeOpusAudioMsg(msg.Raw)
				} else {
					data, err = s.SerializePCMAudioMsg(msg.Raw)
				}
				if err != nil {
					fmt.Println(err)
				} else {
					msg := comms.IOMsg{}
					msg.Topic = ad.AudioToWireTopic
					msg.Data = data
					ad.ToWire <- msg
				}
			}
		}
	}
}

// recordCb is the callback which will be executed each time there is new
// data available on the stream
func (ad *AudioDevice) recordCb(in []float32, iTime portaudio.StreamCallbackTimeInfo, iFlags portaudio.StreamCallbackFlags) {
	switch iFlags {
	case portaudio.InputOverflow:
		fmt.Println("InputOverflow")
		return // data lost, move on!
	}
	// a deep copy is necessary, since portaudio reuses the slice "in"
	buf := make([]float32, len(in))
	for i, v := range in {
		buf[i] = v
	}
	// keep the callback as short as possible
	// sent to raw data to another coroutine for serialization
	msg := comms.IOMsg{}
	msg.Raw = buf
	ad.ToSerialize <- msg
}
