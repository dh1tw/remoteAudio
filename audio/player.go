package audio

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dh1tw/gosamplerate"
	pa "github.com/gordonklaus/portaudio"
	ringBuffer "github.com/zfjagann/golang-ring"
)

// paPlayer implements the audio.Sink interface and is used to store
// the player's internal data / state
type paPlayer struct {
	sync.RWMutex
	options    Options
	deviceInfo *pa.DeviceInfo
	stream     *pa.Stream
	ringMutex  sync.Mutex
	ring       ringBuffer.Ring
	stash      []float32
	volume     float32
	src        src
}

// src contains a samplerate converter and its needed variables
type src struct {
	gosamplerate.Src
	samplerate float64
	ratio      float64
}

// NewPlayer is a factory which returns a new audio player for a specific
// audio output device.
func NewPlayer(opts ...Option) (Sink, error) {

	if err := pa.Initialize(); err != nil {
		return nil, err
	}

	info, err := pa.DefaultOutputDevice()
	if err != nil {
		return nil, err
	}

	player := &paPlayer{
		options: Options{
			DeviceName:      "default",
			Channels:        2,
			Samplerate:      48000,
			FramesPerBuffer: 480,
			RingBufferSize:  10,
			Latency:         time.Millisecond * 10,
		},
		deviceInfo: info,
		ring:       ringBuffer.Ring{},
		volume:     1.0,
	}

	for _, option := range opts {
		option(&player.options)
	}

	// setup a samplerate converter
	srConv, err := gosamplerate.New(gosamplerate.SRC_SINC_FASTEST, player.options.Channels, 65536)
	if err != nil {
		return nil, fmt.Errorf("player: %v", err)
	}

	player.src = src{
		Src:        srConv,
		samplerate: player.options.Samplerate,
		ratio:      1,
	}

	// select Playback Audio Device
	if player.options.DeviceName != "default" {
		device, err := getPaDevice(player.options.DeviceName)
		if err != nil {
			return nil, err
		}
		player.deviceInfo = device
	}

	// setup Audio Stream
	streamDeviceParam := pa.StreamDeviceParameters{
		Device:   player.deviceInfo,
		Channels: player.options.Channels,
		Latency:  player.options.Latency,
	}

	streamParm := pa.StreamParameters{
		FramesPerBuffer: player.options.FramesPerBuffer,
		Output:          streamDeviceParam,
		SampleRate:      player.options.Samplerate,
	}

	// setup ring buffer
	player.ring.SetCapacity(player.options.RingBufferSize)

	stream, err := pa.OpenStream(streamParm, player.playCb)
	if err != nil {
		return nil,
			fmt.Errorf("unable to open playback audio stream on device %s: %s",
				player.options.DeviceName, err)
	}

	player.stream = stream

	return player, nil
}

// pulseaudio callback which will be called continously when the stream is
// started; this function should be short and never block
func (p *paPlayer) playCb(in []float32,
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

	//pull data from Ringbuffer
	p.ringMutex.Lock()
	data := p.ring.Dequeue()
	p.ringMutex.Unlock()

	if data == nil {
		// fill with silence
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

func (p *paPlayer) Start() error {
	if p.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	return p.stream.Start()
}

func (p *paPlayer) Stop() error {
	if p.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	return p.stream.Stop()
}

func (p *paPlayer) Close() error {
	if p.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	p.stream.Abort()
	p.stream.Stop()
	return nil
}

func (p *paPlayer) SetVolume(v float32) {
	p.Lock()
	defer p.Unlock()
	if v < 0 {
		p.volume = 0
		return
	}
	p.volume = v
}

func (p *paPlayer) Volume() float32 {
	p.RLock()
	defer p.RUnlock()
	return p.volume
}

// Enqueue converts the frames in the audio buffer into the right format
// and queues them for playing on the speaker. The token is used to indicate
// if the calling application has to wait before it can enqueue the next
// buffer (e.g. when enqueuing data from a file).
func (p *paPlayer) Enqueue(msg AudioMsg, token Token) {

	var aData []float32
	var err error

	// if necessary adjust the amount of audio channels
	if msg.Channels != p.options.Channels {
		aData = adjustChannels(msg.Channels, p.options.Channels, msg.Data)
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
			log.Println(err)
			return
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
		fmt.Println("stash size:", len(p.stash))
	}

	// if the audio buffer size is actually smaller than the one we need,
	// then stash it for the next time and return
	if len(aData) < expBufferSize {
		p.stash = aData
		return
	}

	// slice of audio buffers which will be enqueued into the ring buffer
	var bData [][]float32

	p.ringMutex.Lock()
	bufCap := p.ring.Capacity()
	bufAvail := bufCap - p.ring.Length()
	p.ringMutex.Unlock()

	// if the aData contains multiples of the expected buffer size,
	// then we chop it into (several) buffers
	if len(aData) >= expBufferSize {
		p.Lock()
		vol := p.volume
		p.Unlock()

		for len(aData) >= expBufferSize {
			if vol != 1 {
				// if necessary, adjust the volume
				adjustVolume(vol, aData[:expBufferSize])
			}
			bData = append(bData, aData[:expBufferSize])
			aData = aData[expBufferSize:]
		}
	}

	// stash the left over
	if len(aData) > 0 {
		p.stash = aData
	}

	// if the msg originates from a stream, we ignore the next statement
	// and move on (which could mean that we overwrite data in the
	// ring buffer - but thats OK to keep latency low)

	// in case we don't have a stream (e.g. writing from a file) and the
	// ring buffer might be full, we have to wait until there is some
	// space available again in the ring buffer
	if !msg.IsStream && bufAvail <= len(bData) {

		token.Add(1)

		go func() {
			for len(bData) > 0 {

				// wait until there is enough space in the ring buffer,
				// or at least 1/2 of the ring buffer is empty again

				for !(bufAvail >= len(bData) || bufAvail >= bufCap/2) {
					time.Sleep(time.Millisecond * 10)
					p.ringMutex.Lock()
					bufAvail = bufCap - p.ring.Length()
					p.ringMutex.Unlock()
				}

				// now we have the space
				p.ringMutex.Lock()
				counter := 0
				for _, frame := range bData {
					p.ring.Enqueue(frame)
					counter++

					bufAvail = bufCap - p.ring.Length()
					if bufAvail == 0 {
						break
					}
				}
				// remove the frames which were enqueued
				bData = bData[counter:]

				// update the available space
				bufAvail = bufCap - p.ring.Length()
				p.ringMutex.Unlock()
			}

			token.Done()
		}()
		return
	}

	p.enqueue(bData, msg.EOF)

	return
}

func (p *paPlayer) enqueue(bData [][]float32, EOF bool) {
	p.ringMutex.Lock()
	defer p.ringMutex.Unlock()
	for _, frame := range bData {
		p.ring.Enqueue(frame)
	}
}

// Flush clears all internal buffers
func (p *paPlayer) Flush() {
	p.ringMutex.Lock()
	defer p.ringMutex.Unlock()

	// delete the stash
	p.stash = []float32{}

	var x interface{}
	// empty the ring buffer
	for x != nil {
		x = p.ring.Dequeue()
	}
}
