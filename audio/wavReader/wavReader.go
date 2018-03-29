package wavReader

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/dh1tw/remoteAudio/audio"
	ga "github.com/go-audio/audio"
	wav "github.com/go-audio/wav"
)

// WavReader implements the audio.Source interface and is used to read (play)
// audio frames from a wav source (e.g. file).
type WavReader struct {
	sync.RWMutex
	options    Options
	buffer     []audio.Msg
	cb         audio.OnDataCb
	isPlaying  bool
	stopPlayCh chan struct{}
}

// NewWavReader reads a wav file from disk into memory and returns a
// WavReader object which implements the audio.Source interface.
func NewWavReader(file string, opts ...Option) (*WavReader, error) {

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := wav.NewDecoder(f)

	if !dec.IsValidFile() {
		return nil, errors.New("invalid WAV file")
	}

	w := WavReader{
		buffer: []audio.Msg{},
		options: Options{
			FramesPerBuffer: DefaultFramesPerBuffer,
		},
	}

	for _, o := range opts {
		o(&w.options)
	}

	buf := &ga.IntBuffer{
		Data:   make([]int, w.options.FramesPerBuffer),
		Format: dec.Format(),
	}

	for err == nil {
		n, err := dec.PCMBuffer(buf)
		if err != nil {
			return nil, err
		}

		if n == 0 {
			break
		}

		if n != len(buf.Data) {
			buf.Data = buf.Data[:n]
		}

		msg := audio.Msg{
			Data:       buf.AsFloat32Buffer().Data,
			Channels:   buf.Format.NumChannels,
			Samplerate: float64(buf.Format.SampleRate),
			Frames:     n,
		}
		w.buffer = append(w.buffer, msg)
	}

	w.buffer[len(w.buffer)-1].EOF = true

	return &w, nil
}

// SetCb sets the callback which will be executed to provide audio buffers.
func (w *WavReader) SetCb(cb audio.OnDataCb) {
	w.Lock()
	defer w.Unlock()
	w.cb = cb
}

// Start will "play" the audio by providing audio buffers through the
// set callback function.
func (w *WavReader) Start() error {

	w.Lock()
	defer w.Unlock()

	if w.isPlaying {
		return nil
	}

	w.stopPlayCh = make(chan struct{})

	go w.play(w.buffer, w.stopPlayCh, w.cb)
	w.isPlaying = true

	return nil
}

func (w *WavReader) play(audioMsgs []audio.Msg, stopCh chan struct{}, cb audio.OnDataCb) {

	if cb == nil {
		log.Println("wavReader callback not set")
		return
	}

	for _, msg := range audioMsgs {

		// calculate duration in milliseconds of one frame. If they are
		// stereo the channels are interleaved. The duration is shorted
		// by 5ms to avoid empty buffers.
		duration := (msg.Frames/msg.Channels)/int(msg.Samplerate/1000) - 5

		select {
		case <-stopCh:
			return
		case <-time.After(time.Duration(duration) * time.Millisecond):
			cb(msg)
		}

	}
	close(stopCh)
	w.Lock()
	defer w.Unlock()
	w.isPlaying = false
}

// Stop cancels sending audio through the callback.
func (w *WavReader) Stop() error {
	w.Lock()
	defer w.Unlock()

	if w.isPlaying {
		fmt.Println("is still playing")
		close(w.stopPlayCh)
	}
	w.isPlaying = false

	return nil
}

// Close shutsdown the wav player
func (w *WavReader) Close() error {
	return nil
}
