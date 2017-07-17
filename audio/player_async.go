package audio

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dh1tw/gosamplerate"
	"github.com/dh1tw/remoteAudio/events"
	"github.com/gordonklaus/portaudio"
	"github.com/spf13/viper"
	ringBuffer "github.com/zfjagann/golang-ring"
	"gopkg.in/hraban/opus.v2"
)

//PlayerASync plays received audio on a local audio device asynchronously
func PlayerASync(ad AudioDevice) {

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	defer ad.WaitGroup.Done()

	portaudio.Initialize()
	defer portaudio.Terminate()

	// Subscribe on events
	shutdownCh := ad.Events.Sub(events.Shutdown)

	// give Portaudio a few milliseconds to initialize
	// this is necessary to avoid a SIGSEGV in case
	// DefaultOutputDevice is accessed without portaudio
	// being completely initialized
	time.Sleep(time.Millisecond * 300)

	ad.out = make([]float32, ad.FramesPerBuffer*ad.Channels)

	//ad.out doesn't need to be initialized with a fixed buffer size
	//since the slice will be copied from the incoming data
	//and will therefore replay any buffer size

	var deviceInfo *portaudio.DeviceInfo
	var err error

	// select Playback Audio Device
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
		deviceInfo = ad.Device
	}

	// setup Audio Stream
	streamDeviceParam := portaudio.StreamDeviceParameters{
		Device:   deviceInfo,
		Channels: ad.Channels,
		Latency:  ad.Latency,
	}

	streamParm := portaudio.StreamParameters{
		FramesPerBuffer: ad.FramesPerBuffer,
		// FramesPerBuffer: portaudio.FramesPerBufferUnspecified,
		Output:     streamDeviceParam,
		SampleRate: ad.Samplingrate,
	}

	var stream *portaudio.Stream

	audioBufferSize := viper.GetInt("audio.rx-buffer-length")

	// the deserializer struct is mainly used to cache variables which have
	// to be read / set during the deserialization
	var d deserializer
	d.AudioDevice = &ad
	d.txTimestamp = time.Now()
	// initialize audio (ring) buffer
	d.muRing = sync.Mutex{}
	d.ring = ringBuffer.Ring{}
	d.ring.SetCapacity(audioBufferSize)

	// default volume - replay as is
	d.volume = 1.0

	// initialize the Opus Decoder
	opusDecoder, err := opus.NewDecoder(int(ad.Samplingrate), ad.AudioStream.Channels)

	if err != nil || opusDecoder == nil {
		fmt.Println(err)
		return
	}
	d.opusDecoder = opusDecoder
	d.opusBuffer = make([]float32, 520000) //max opus message size

	// open the audio stream
	stream, err = portaudio.OpenStream(streamParm, d.playCb)
	if err != nil {
		fmt.Printf("unable to open playback audio stream on device %s\n", ad.DeviceName)
		fmt.Println(err)
		return // exit go routine
	}
	defer stream.Close()

	// create the PCM samplerate converter
	ad.PCMSamplerateConverter, err = gosamplerate.New(viper.GetInt("output-device.quality"), ad.Channels, 65536)
	if err != nil {
		fmt.Println("unable to create resampler")
		fmt.Println(err)
		return // exit go routine
	}
	defer gosamplerate.Delete(ad.PCMSamplerateConverter)

	// Start the playback audio stream
	if err = stream.Start(); err != nil {
		fmt.Printf("unable to start playback audio stream on device %s\n", ad.DeviceName)
		fmt.Println(err)
		return // exit go routine
	}
	defer stream.Stop()

	bufferFrameSizeChangeCh := ad.Events.Sub(events.NewAudioFrameSize)
	setVolumeCh := ad.Events.Sub(events.SetVolume)

	// cache holding the id of user from which we currently receive audio
	txUser := ""

	// Tickers to check if we still receive audio from a certain user.
	// This is needed on the server to release the "lock" and allow
	// others to transmit
	txUserResetTicker := time.NewTicker(100 * time.Millisecond)

	// Everything has been set up, let's start execution
	ad.Events.Pub(true, events.ForwardAudio)

	for {
		select {

		// shutdown application gracefully
		case <-shutdownCh:
			log.Println("Shutdown Player")
			return

		// Used in the Server
		case <-txUserResetTicker.C:
			d.muTx.Lock()
			// clear the tx user lock if nobody transmitted during the last 500ms
			if time.Since(d.txTimestamp) > 500*time.Millisecond {
				d.txUser = ""
			}

			// check if the tx user has changed and publish changes
			if txUser != d.txUser {
				ad.Events.Pub(d.txUser, events.TxUser)
				txUser = d.txUser
			}
			d.muTx.Unlock()

		case ev := <-setVolumeCh:
			volume := ev.(float32)
			d.muTx.Lock()
			d.volume = volume
			d.muTx.Unlock()

		// When the size of the received audio frame is different from our
		// buffer size, we have to resize and restart the stream. This causes
		// additional latency on startup! Should be avoided.
		case ev := <-bufferFrameSizeChangeCh:
			ad.Events.Pub(false, events.ForwardAudio)
			newBufSize := ev.(int)
			stream.Abort()
			stream.Close()
			log.Printf("WARNING: Samplerate has changed from %d, to %d", len(ad.out), newBufSize)
			// new buffer with new size
			ad.out = make([]float32, newBufSize)
			// update stream parameters
			streamParm.FramesPerBuffer = newBufSize / ad.Channels

			stream, err = portaudio.OpenStream(streamParm, d.playCb)
			if err != nil {
				fmt.Printf("unable to open playback audio stream on device %s\n", ad.DeviceName)
				fmt.Println(err)
				return
			}
			// d.PCMSamplerateConverter.Reset()
			if err = stream.Start(); err != nil {
				fmt.Printf("unable to start playback audio stream on device %s\n", ad.DeviceName)
				fmt.Println(err)
				return
			}
			// lets forward again audio streams received via Network
			ad.Events.Pub(true, events.ForwardAudio)

		// deserialize and write received audio data into the ring buffer
		case msg := <-ad.ToDeserialize:
			err := d.DeserializeAudioMsg(msg)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

// playCb is the playback which is called by portaudio to write the audio
// samples to the speaker
func (d *deserializer) playCb(in []float32, iTime portaudio.StreamCallbackTimeInfo, iFlags portaudio.StreamCallbackFlags) {
	switch iFlags {
	case portaudio.OutputUnderflow:
		fmt.Println("OutputUnderflow")
		return // move on!
	case portaudio.OutputOverflow:
		fmt.Println("OutputOverflow")
		return // move on!
	}

	//pull data from Ringbuffer
	d.muRing.Lock()
	data := d.ring.Dequeue()
	d.muRing.Unlock()

	if data != nil {
		audioData := data.([]float32)
		if len(audioData) == len(in) {
			copy(in, audioData) //copy data into buffer
		}
		return
	}

	// fill with silence
	for i := 0; i < len(in); i++ {
		in[i] = 0
	}
}
