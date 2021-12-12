package scWriter

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"

	ringBuffer "github.com/dh1tw/golang-ring"
	"github.com/dh1tw/gosamplerate"
	"github.com/dh1tw/remoteAudio/audio"
	pa "github.com/gordonklaus/portaudio"
)

// ScWriter implements the audio.Sink interface and is used to write (play)
// audio on a local audio output device (e.g. speakers).
type ScWriter struct {
	sync.RWMutex
	options    Options
	deviceInfo *pa.DeviceInfo
	stream     *pa.Stream
	ring       ringBuffer.Ring
	stash      []float32
	volume     float32
	src        src
	bufFill    bool // indicates if the buffer is filling up
}

// src contains a samplerate converter and its needed variables
type src struct {
	gosamplerate.Src
	samplerate float64
	ratio      float64
}

// NewScWriter returns a new soundcard writer for a specific audio output
// device. This is typically a speaker or a pair of headphones.
func NewScWriter(opts ...Option) (*ScWriter, error) {

	w := &ScWriter{
		options: Options{
			DeviceName:      "default",
			HostAPI:         "default",
			Channels:        2,
			Samplerate:      48000,
			FramesPerBuffer: 480,
			RingBufferSize:  10,
			Latency:         time.Millisecond * 10,
		},
		deviceInfo: nil,
		ring:       ringBuffer.Ring{},
		volume:     0.7,
	}

	for _, option := range opts {
		option(&w.options)
	}

	// setup a samplerate converter
	srConv, err := gosamplerate.New(gosamplerate.SRC_SINC_FASTEST, w.options.Channels, 65536)
	if err != nil {
		return nil, fmt.Errorf("player: %v", err)
	}

	w.src = src{
		Src:        srConv,
		samplerate: w.options.Samplerate,
		ratio:      1,
	}

	var hostAPI *pa.HostApiInfo

	if w.options.HostAPI == "default" {
		switch runtime.GOOS {
		case "windows":
			// try to use WASAPI since it provides lower latency than the
			// other windows audio apis
			ha, err := pa.HostApi(pa.WASAPI)
			if err != nil {
				// try to fallback to the default API
				ha, err = pa.DefaultHostApi()
				if err != nil {
					return nil, fmt.Errorf("unable to determine the default host api - please provide a specific host api")
				}
			}
			hostAPI = ha
		default:
			// all other OS
			ha, err := pa.DefaultHostApi()
			if err != nil {
				return nil, fmt.Errorf("unable to determine the default host api - please provide a specific host api")
			}
			hostAPI = ha
		}
	} else {
		// non-default HostAPI
		ha, err := getHostAPI(w.options.HostAPI)
		if err != nil {
			return nil, err
		}
		hostAPI = ha
	}

	if w.options.DeviceName == "default" {
		w.deviceInfo = hostAPI.DefaultOutputDevice
	} else {
		dev, err := getPaDevice(w.options.DeviceName, hostAPI)
		if err != nil {
			return nil, err
		}
		w.deviceInfo = dev
	}

	// setup Audio Stream
	streamDeviceParam := pa.StreamDeviceParameters{
		Device:   w.deviceInfo,
		Channels: w.options.Channels,
		Latency:  w.options.Latency,
	}

	streamParm := pa.StreamParameters{
		FramesPerBuffer: w.options.FramesPerBuffer,
		Output:          streamDeviceParam,
		SampleRate:      w.options.Samplerate,
	}

	// setup ring buffer
	w.ring.SetCapacity(w.options.RingBufferSize)

	stream, err := pa.OpenStream(streamParm, w.playCb)
	if err != nil {
		return nil,
			fmt.Errorf("unable to open playback audio stream on device %s: %s",
				w.options.DeviceName, err)
	}

	w.stream = stream
	log.Printf("output sound device: %s, HostAPI: %s\n", w.deviceInfo.Name, w.deviceInfo.HostApi.Name)

	return w, nil
}

// portaudio callback which will be called continuously when the stream is
// started; this function should be short and never block
func (p *ScWriter) playCb(in []float32,
	iTime pa.StreamCallbackTimeInfo,
	iFlags pa.StreamCallbackFlags) {
	switch iFlags {
	case pa.OutputUnderflow:
		log.Println("Output Underflow")
		return // move on!
	case pa.OutputOverflow:
		log.Println("Output Overflow")
		return // move on!
	}

	var data interface{}

	p.Lock()
	bufFill := p.bufFill
	bufCapacity := p.ring.Capacity()
	bufLength := p.ring.Length()
	// when filling up the buffer, don't dequeue data
	if !bufFill {
		//pull data from Ringbuffer
		data = p.ring.Dequeue()
	}
	p.Unlock()

	// log.Printf("Buf: %d / %d\n", bufLength, bufCapacity)

	// start filling buffer when buffer runs empty
	if bufLength == 0 {
		p.Lock()
		p.bufFill = true
		p.Unlock()
	}

	if bufFill {
		// stop filling buffer when it's again half full
		if bufLength >= bufCapacity/2 {
			p.bufFill = false
		}
	}

	// if no data is available we fill the audio package with silence
	if data == nil {
		for i := 0; i < len(in); i++ {
			in[i] = 0
		}
		return
	}

	audioData := data.([]float32)

	// should never happen
	if len(audioData) != len(in) {
		log.Printf("unable to play audio frame; expected frame size %d, but got %d",
			len(in), len(audioData))
		return
	}

	//copy data into buffer
	copy(in, audioData)
}

// Start starts streaming audio to the Soundcard output device (e.g. Speaker).
func (p *ScWriter) Start() error {
	if p.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	return p.stream.Start()
}

// Stop stops streaming audio.
func (p *ScWriter) Stop() error {
	if p.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	return p.stream.Stop()
}

// Close shutsdown properly the soundcard audio device.
func (p *ScWriter) Close() error {
	if p.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	p.stream.Abort()
	p.stream.Stop()
	return nil
}

// SetVolume sets the volume for all upcoming audio frames.
func (p *ScWriter) SetVolume(v float32) {
	p.Lock()
	defer p.Unlock()
	if v < 0 {
		p.volume = 0
	} else if v > 1 {
		p.volume = 1
	} else {
		p.volume = v
	}
}

// Volume returns the current volume.
func (p *ScWriter) Volume() float32 {
	p.RLock()
	defer p.RUnlock()
	return p.volume
}

// Write converts the frames in the audio buffer into the right format
// and queues them into a ring buffer for playing on the speaker.
func (p *ScWriter) Write(msg audio.Msg) error {

	var aData []float32
	var err error

	// if necessary adjust the amount of audio channels
	if msg.Channels != p.options.Channels {
		aData = audio.AdjustChannels(msg.Channels, p.options.Channels, msg.Data)
	} else {
		aData = msg.Data
	}

	// if necessary, resample the audio
	if msg.Samplerate != p.options.Samplerate {
		if p.src.samplerate != msg.Samplerate {
			p.src.Reset()
			p.src.samplerate = msg.Samplerate
			p.src.ratio = p.options.Samplerate / msg.Samplerate
		}
		aData, err = p.src.Process(aData, p.src.ratio, false)
		if err != nil {
			return err
		}
	}

	// audio buffer size we want to write into our ring buffer
	// (size expected by portaudio callback)
	expBufferSize := p.options.FramesPerBuffer * p.options.Channels

	// if there is data stashed from previous calles, get it and prepend it
	// to the data received
	if len(p.stash) > 0 {
		aData = append(p.stash, aData...)
		p.stash = p.stash[:0] // empty
	}

	if msg.EOF {
		// get the stuff from the stash
		fmt.Println("EOF!!!")
	}

	// if the audio buffer size is actually smaller than the one we need,
	// then stash it for the next time and return
	if len(aData) < expBufferSize {
		p.stash = aData
		return nil
	}

	// slice of audio buffers which will be enqueued into the ring buffer
	var bData [][]float32

	// if the aData contains multiples of the expected buffer size,
	// then we chop it into (several) buffers
	if len(aData) >= expBufferSize {
		p.Lock()
		vol := p.volume
		p.Unlock()

		for len(aData) >= expBufferSize {
			if vol != 1 {
				// if necessary, adjust the volume
				audio.AdjustVolume(vol, aData[:expBufferSize])
			}
			bData = append(bData, aData[:expBufferSize])
			aData = aData[expBufferSize:]
		}
	}

	// stash the left over
	if len(aData) > 0 {
		p.stash = aData
	}

	p.enqueue(bData, msg.EOF)

	return nil
}

func (p *ScWriter) enqueue(bData [][]float32, EOF bool) {
	p.Lock()
	defer p.Unlock()
	for _, frame := range bData {
		p.ring.Enqueue(frame)
	}
}

// Flush clears all internal buffers
func (p *ScWriter) Flush() {
	p.Lock()
	defer p.Unlock()

	// delete the stash
	p.stash = []float32{}

	p.ring = ringBuffer.Ring{}
	p.ring.SetCapacity(p.options.RingBufferSize)
}

// getHostAPI takes the name of a supported portaudio host api and returns
// the corresponding portaudio hostApiInfo object
func getHostAPI(name string) (*pa.HostApiInfo, error) {

	var hostAPIType pa.HostApiType

	switch strings.ToLower(name) {
	case "indevelopment":
		hostAPIType = pa.InDevelopment
	case "directsound":
		hostAPIType = pa.DirectSound
	case "mme":
		hostAPIType = pa.MME
	case "asio":
		hostAPIType = pa.ASIO
	case "soundmanager":
		hostAPIType = pa.SoundManager
	case "coreaudio":
		hostAPIType = pa.CoreAudio
	case "oss":
		hostAPIType = pa.OSS
	case "alsa":
		hostAPIType = pa.ALSA
	case "al":
		hostAPIType = pa.AL
	case "beos":
		hostAPIType = pa.BeOS
	case "wdmks":
		hostAPIType = pa.WDMkS
	case "jack":
		hostAPIType = pa.JACK
	case "wasapi":
		hostAPIType = pa.WASAPI
	case "audiosciencehpi":
		hostAPIType = pa.AudioScienceHPI
	default:
		return nil, fmt.Errorf("unknown host api type: %s", name)
	}

	hostAPIInfo, err := pa.HostApi(hostAPIType)
	if err != nil {
		return nil, fmt.Errorf("unable to load host api %s: %s", name, err.Error())
	}

	return hostAPIInfo, nil

}

// getPaDevice checks if the Audio Devices actually exist and
// then returns it
func getPaDevice(name string, hostAPI *pa.HostApiInfo) (*pa.DeviceInfo, error) {
	for _, device := range hostAPI.Devices {
		if strings.ToLower(device.Name) == strings.ToLower(name) {
			return device, nil
		}
	}
	return nil, fmt.Errorf("unknown audio device '%s'", name)
}

// Write converts the frames in the audio buffer into the right format
// and queues them into a ring buffer for playing on the speaker. The token is
// used to indicate if the calling application has to wait before it can
// enqueue the next buffer.
// func (p *ScWriter) Write(msg audio.Msg, token audio.Token) error {

// 	var aData []float32
// 	var err error

// 	// if necessary adjust the amount of audio channels
// 	if msg.Channels != p.options.Channels {
// 		aData = audio.AdjustChannels(msg.Channels, p.options.Channels, msg.Data)
// 	} else {
// 		aData = msg.Data
// 	}

// 	// if necessary, resample the audio
// 	if msg.Samplerate != p.options.Samplerate {
// 		if p.src.samplerate != msg.Samplerate {
// 			p.src.Reset()
// 			p.src.samplerate = msg.Samplerate
// 			p.src.ratio = p.options.Samplerate / msg.Samplerate
// 		}
// 		aData, err = p.src.Process(aData, p.src.ratio, false)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	// audio buffer size we want to write into our ring buffer
// 	// (size expected by portaudio callback)
// 	expBufferSize := p.options.FramesPerBuffer * p.options.Channels

// 	// if there is data stashed from previous calles, get it and prepend it
// 	// to the data received
// 	if len(p.stash) > 0 {
// 		aData = append(p.stash, aData...)
// 		p.stash = p.stash[:0] // empty
// 	}

// 	if msg.EOF {
// 		// get the stuff from the stash
// 		fmt.Println("EOF!!!")
// 		fmt.Println("stash size:", len(p.stash))
// 	}

// 	// if the audio buffer size is actually smaller than the one we need,
// 	// then stash it for the next time and return
// 	if len(aData) < expBufferSize {
// 		p.stash = aData
// 		return nil
// 	}

// 	// slice of audio buffers which will be enqueued into the ring buffer
// 	var bData [][]float32

// 	p.Lock()
// 	bufCap := p.ring.Capacity()
// 	bufAvail := bufCap - p.ring.Length()
// 	p.Unlock()

// 	// if the aData contains multiples of the expected buffer size,
// 	// then we chop it into (several) buffers
// 	if len(aData) >= expBufferSize {
// 		p.Lock()
// 		vol := p.volume
// 		p.Unlock()

// 		for len(aData) >= expBufferSize {
// 			if vol != 1 {
// 				// if necessary, adjust the volume
// 				audio.AdjustVolume(vol, aData[:expBufferSize])
// 			}
// 			bData = append(bData, aData[:expBufferSize])
// 			aData = aData[expBufferSize:]
// 		}
// 	}

// 	// stash the left over
// 	if len(aData) > 0 {
// 		p.stash = aData
// 	}

// 	// if the msg originates from a stream, we ignore the next statement
// 	// and move on (which could mean that we overwrite data in the
// 	// ring buffer - but thats OK to keep latency low)

// 	// in case we don't have a stream (e.g. writing from a file) and the
// 	// ring buffer might be full, we have to wait until there is some
// 	// space available again in the ring buffer
// 	if !msg.IsStream && bufAvail <= len(bData) {

// 		token.Add(1)

// 		go func() {
// 			for len(bData) > 0 {

// 				// wait until there is enough space in the ring buffer,
// 				// or at least 1/2 of the ring buffer is empty again

// 				for !(bufAvail >= len(bData) || bufAvail >= bufCap/2) {
// 					time.Sleep(time.Millisecond * 10)
// 					p.Lock()
// 					bufAvail = bufCap - p.ring.Length()
// 					p.Unlock()
// 				}

// 				// now we have the space
// 				p.Lock()
// 				counter := 0
// 				for _, frame := range bData {
// 					p.ring.Enqueue(frame)
// 					counter++

// 					bufAvail = bufCap - p.ring.Length()
// 					if bufAvail == 0 {
// 						break
// 					}
// 				}
// 				// remove the frames which were enqueued
// 				bData = bData[counter:]

// 				// update the available space
// 				bufAvail = bufCap - p.ring.Length()
// 				p.Unlock()
// 			}

// 			token.Done()
// 		}()
// 		return nil
// 	}

// 	p.enqueue(bData, msg.EOF)

// 	return nil
// }
