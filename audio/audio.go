package audio

import (
	"sync"
)

// Source is the interface which is implemented by an audio source. This
// could be streaming data received from a network connection, a local
// audio source (e.g. microphone) or a audio read from a local file.
type Source interface {
	Start() error
	Stop() error
	Close() error
	// read callback when data available
}

// Sink is the interface which is implemented by an audio sink. This could
// be an Audio player or a file for recording.
type Sink interface {
	Start() error
	Stop() error
	Close() error
	SetVolume(float32)
	Volume() float32
	Enqueue(AudioMsg, Token)
	Flush()
}

// Token contains a sync.Waitgroup and is used with an audio sink. The
// token will indicate the application to wait until further audio buffers
// can be enqueued into the sink.
type Token struct {
	*sync.WaitGroup
}

// NewToken is a convinience constructor for a Token.
func NewToken() Token {
	t := Token{&sync.WaitGroup{}}
	return t
}

// AudioMsg contains an audio buffer with it's metadata
type AudioMsg struct {
	Data       []float32
	Samplerate float64
	// Bitdepth   int
	Channels int
	Frames   int // Number of Frames in the buffer
	IsStream bool
	EOF      bool // End of File
}
