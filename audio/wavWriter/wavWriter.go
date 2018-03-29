package wavWriter

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/dh1tw/gosamplerate"
	"github.com/dh1tw/remoteAudio/audio"
	ga "github.com/go-audio/audio"
	wav "github.com/go-audio/wav"
)

// WavWriter implements the audio.Sink interface and is used to write (record)
// audio frames in the wav format.
type WavWriter struct {
	sync.Mutex
	file    *os.File
	encoder *wav.Encoder
	options Options
	volume  float32
	src     src
}

// src contains a samplerate converter and its needed variables
type src struct {
	gosamplerate.Src
	samplerate float64
	ratio      float64
}

// NewWavWriter returns a wavWriter to which audio frames can be written to.
// The audio data will be saved in the wav format.
func NewWavWriter(path string, opts ...Option) (*WavWriter, error) {

	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	w := &WavWriter{
		options: Options{
			Channels:   DefaultChannels,
			BitDepth:   DefaultBitDepth,
			Samplerate: DefaultSamplerate,
		},
		volume: 1.0,
		file:   f,
	}

	for _, o := range opts {
		o(&w.options)
	}

	// make sure we only allow 12 / 16 bit Bitdepth (dynamic range)
	switch w.options.BitDepth {
	case 12, 16:
	default:
		w.options.BitDepth = 16
	}

	// setup a samplerate converter
	srConv, err := gosamplerate.New(gosamplerate.SRC_SINC_FASTEST,
		w.options.Channels, 65536)
	if err != nil {
		return nil, fmt.Errorf("WavWriter samplerate converter: %v", err)
	}
	w.src = src{
		Src:        srConv,
		samplerate: w.options.Samplerate,
		ratio:      1,
	}

	w.encoder = wav.NewEncoder(f, int(w.options.Samplerate),
		w.options.BitDepth, w.options.Channels, 1)

	return w, nil
}

// Start writing audio to the wav file.
func (w *WavWriter) Start() error {
	return nil
}

// Stop writing audio frames to the wav file.
func (w *WavWriter) Stop() error {
	return nil
}

// Close shuts down properly the wavWriter.
func (w *WavWriter) Close() error {
	err := w.encoder.Close()
	w.file.Close()
	return err
}

// SetVolume sets the volume for all incoming audio frames.
func (w *WavWriter) SetVolume(v float32) {
	w.Lock()
	defer w.Unlock()
	if v < 0 {
		w.volume = 0
	} else if v > 1 {
		w.volume = 1
	} else {
		w.volume = v
	}
}

// Volume returns the current volume.
func (w *WavWriter) Volume() float32 {
	w.Lock()
	defer w.Unlock()
	return w.volume
}

// Write enqueues audio buffers to be written into the wav file. Channels
// and Samplerate will be adjusted, if necessary.
func (w *WavWriter) Write(msg audio.Msg) error {

	var aData []float32
	var err error

	// max size of an audio sample converted from float32 to int16 or int32
	const (
		b12 int = 4096
		b16 int = 32768
		// b32 int = 2147483648 //to large on 32bit ARM
	)

	// if necessary adjust the amount of audio channels
	if msg.Channels != w.options.Channels {
		aData = audio.AdjustChannels(msg.Channels, w.options.Channels, msg.Data)
	} else {
		aData = msg.Data
	}

	w.Lock()
	audio.AdjustVolume(w.volume, aData)
	w.Unlock()

	if msg.Samplerate != w.options.Samplerate {
		if w.src.samplerate != msg.Samplerate {
			w.src.Reset()
			w.src.samplerate = msg.Samplerate
			w.src.ratio = w.options.Samplerate / msg.Samplerate
		}
		aData, err = w.src.Process(aData, w.src.ratio, false)
		if err != nil {
			return err
		}
	}

	buf := ga.IntBuffer{
		Format: &ga.Format{
			SampleRate:  int(w.options.Samplerate),
			NumChannels: w.options.Channels,
		},
	}

	// prepare the bitdepth / dynamic range
	var max int
	switch w.options.BitDepth {
	case 12:
		max = b12
	// case 32:
	// 	max = b32
	default:
		max = b16
	}

	for _, frame := range aData {
		f := int(frame * float32(max))
		if f > max-1 {
			buf.Data = append(buf.Data, max)
		} else if f < -max {
			buf.Data = append(buf.Data, -max)
		} else {
			buf.Data = append(buf.Data, f)
		}
	}

	if err := w.encoder.Write(&buf); err != nil {
		log.Println(err)
	}

	fmt.Printf("writing %v samples\n", w.encoder.WrittenBytes)

	return nil
}

// Flush is not implemented
func (w *WavWriter) Flush() {}
