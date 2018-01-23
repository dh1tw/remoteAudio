package wavReader

import (
	"errors"
	"os"

	"github.com/dh1tw/remoteAudio/audio"
	ga "github.com/go-audio/audio"
	wav "github.com/go-audio/wav"
)

// WavReader implements the audio.Source interface and is used to read (play)
// audio frames from a wav source (e.g. file).
type WavReader struct {
	options   Options
	buffer    []audio.AudioMsg
	cb        audio.OnDataCb
	isPlaying bool
}

// NewWavReader reads a wav file from disk into memory and returns a
// WavReader object which implements the audio.Source interface.
func NewWavReader(path string, opts ...Option) (*WavReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	dec := wav.NewDecoder(file)

	if !dec.IsValidFile() {
		return nil, errors.New("invalid WAV file")
	}

	w := WavReader{
		buffer: []audio.AudioMsg{},
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

		msg := audio.AudioMsg{
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
	w.cb = cb
}

// Start will "play" the audio by providing audio buffers through the
// set callback function.
func (w *WavReader) Start() error {

	if w.isPlaying {
		return nil
	}

	w.isPlaying = true

	// TBD use mutex!
	go func() {
		for _, msg := range w.buffer {
			if w.cb != nil {
				w.cb(msg)
			}
		}
		w.isPlaying = false
	}()

	return nil
}

// Stop cancels sending audio through the callback.
func (w *WavReader) Stop() error {
	w.cb = nil
	return nil
}

// Close shutsdown the wav player
func (w *WavReader) Close() error {
	return nil
}
