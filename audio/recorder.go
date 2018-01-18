package audio

import (
	"fmt"
	"log"
	"sync"
	"time"

	pa "github.com/gordonklaus/portaudio"
)

type paRecorder struct {
	sync.RWMutex
	options     Options
	deviceInfo  *pa.DeviceInfo
	stream      *pa.Stream
	newDataHdlr func(AudioMsg)
}

// NewRecorder returns an initialized recorder which will steam audio
// asynchronously from an a local audio device (e.g. a microphone)
func NewRecorder(hdlr func(AudioMsg), opts ...Option) (Source, error) {

	if err := pa.Initialize(); err != nil {
		return nil, err
	}

	info, err := pa.DefaultInputDevice()
	if err != nil {
		return nil, err
	}

	rec := &paRecorder{
		options: Options{
			DeviceName:      "default",
			Channels:        1,
			Samplerate:      48000,
			FramesPerBuffer: 480,
			Latency:         time.Millisecond * 10,
		},
		newDataHdlr: hdlr,
		deviceInfo:  info,
	}

	for _, option := range opts {
		option(&rec.options)
	}

	if rec.options.DeviceName != "default" {
		device, err := getPaDevice(rec.options.DeviceName)
		if err != nil {
			return nil, err
		}
		rec.deviceInfo = device
	}

	// setup Audio Stream
	streamDeviceParam := pa.StreamDeviceParameters{
		Device:   rec.deviceInfo,
		Channels: rec.options.Channels,
		Latency:  rec.options.Latency,
	}

	streamParm := pa.StreamParameters{
		FramesPerBuffer: rec.options.FramesPerBuffer,
		Input:           streamDeviceParam,
		SampleRate:      rec.options.Samplerate,
	}

	stream, err := pa.OpenStream(streamParm, rec.recordCb)
	if err != nil {
		return nil,
			fmt.Errorf("unable to open recording audio stream on device %s: %s",
				rec.deviceInfo.Name, err)
	}
	rec.stream = stream

	return rec, nil
}

// recordCb is the callback which will be executed each time there is new
// data available on the stream
func (r *paRecorder) recordCb(in []float32,
	iTime pa.StreamCallbackTimeInfo,
	iFlags pa.StreamCallbackFlags) {

	if iFlags == pa.InputOverflow {
		log.Println("InputOverflow")
		return // data lost, move on!
	}

	if r.newDataHdlr == nil {
		return
	}

	// a deep copy is necessary, since portaudio reuses the slice "in"
	buf := make([]float32, len(in))
	for i, v := range in {
		buf[i] = v
	}

	msg := AudioMsg{
		Data:       buf,
		Samplerate: r.options.Samplerate,
		Channels:   r.options.Channels,
		Frames:     r.options.FramesPerBuffer,
		IsStream:   true,
	}

	// execute the callback for further processing
	r.newDataHdlr(msg)

}

func (r *paRecorder) Start() error {
	if r.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	return r.stream.Start()
}

func (r *paRecorder) Stop() error {
	if r.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	return r.stream.Stop()
}

func (r *paRecorder) Close() error {
	if r.stream == nil {
		return fmt.Errorf("portaudio stream not initialized")
	}
	r.stream.Abort()
	r.stream.Stop()
	return nil
}
