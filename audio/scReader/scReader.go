package scReader

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/dh1tw/remoteAudio/audio"
	pa "github.com/gordonklaus/portaudio"
)

// ScReader implements the audio.Source interface and is used to read (record)
// audio from a local sound card (e.g. microphone).
type ScReader struct {
	sync.RWMutex
	options    Options
	deviceInfo *pa.DeviceInfo
	stream     *pa.Stream
	cb         func(audio.Msg)
}

// NewScReader returns a soundcard reader which steams audio
// asynchronously from an a local audio device (e.g. a microphone).
func NewScReader(opts ...Option) (*ScReader, error) {

	if err := pa.Initialize(); err != nil {
		return nil, err
	}

	info, err := pa.DefaultInputDevice()
	if err != nil {
		return nil, err
	}

	r := &ScReader{
		options: Options{
			DeviceName:      "default",
			Channels:        1,
			Samplerate:      48000,
			FramesPerBuffer: 480,
			Latency:         time.Millisecond * 10,
		},
		deviceInfo: info,
	}

	for _, option := range opts {
		option(&r.options)
	}

	if r.options.DeviceName != "default" {
		device, err := getPaDevice(r.options.DeviceName)
		if err != nil {
			return nil, err
		}
		r.deviceInfo = device
	}

	// setup Audio Stream
	streamDeviceParam := pa.StreamDeviceParameters{
		Device:   r.deviceInfo,
		Channels: r.options.Channels,
		Latency:  r.options.Latency,
	}

	streamParm := pa.StreamParameters{
		FramesPerBuffer: r.options.FramesPerBuffer,
		Input:           streamDeviceParam,
		SampleRate:      r.options.Samplerate,
	}

	stream, err := pa.OpenStream(streamParm, r.paReadCb)
	if err != nil {
		return nil,
			fmt.Errorf("unable to open recording audio stream on device %s: %s",
				r.deviceInfo.Name, err)
	}
	r.stream = stream

	return r, nil
}

// SetCb sets the callback which will be executed to provide audio buffers.
func (r *ScReader) SetCb(cb audio.OnDataCb) {
	r.cb = cb
}

// paReadCb is the callback which will be executed each time there is new
// data available on the stream
func (r *ScReader) paReadCb(in []float32,
	iTime pa.StreamCallbackTimeInfo,
	iFlags pa.StreamCallbackFlags) {

	if iFlags == pa.InputOverflow {
		log.Println("InputOverflow")
		return // data lost, move on!
	}

	if r.cb == nil {
		return
	}

	// a deep copy is necessary, since portaudio reuses the slice "in"
	buf := make([]float32, len(in))
	for i, v := range in {
		buf[i] = v
	}

	msg := audio.Msg{
		Data:       buf,
		Samplerate: r.options.Samplerate,
		Channels:   r.options.Channels,
		Frames:     r.options.FramesPerBuffer,
		IsStream:   true,
	}

	// execute the callback for further processing
	r.cb(msg)
}

// Start will start streaming audio from a local soundcard device.
// The read audio buffers will be provided through the callback.
func (r *ScReader) Start() error {
	if r.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	return r.stream.Start()
}

// Stop stops streaming audio.
func (r *ScReader) Stop() error {
	if r.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	return r.stream.Stop()
}

// Close shutsdown properly the soundcard reader.
func (r *ScReader) Close() error {
	if r.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	r.stream.Abort()
	r.stream.Stop()
	return nil
}

// getPaDevice checks if the Audio Devices actually exist and
// then returns it
func getPaDevice(name string) (*pa.DeviceInfo, error) {
	devices, _ := pa.Devices()
	for _, device := range devices {
		if device.Name == name {
			return device, nil
		}
	}
	return nil, fmt.Errorf("unknown audio device %s", name)
}
