package audio

import "sync"

// OnDataCb is a callback function which will be called by an audio source
// when new data is available
type OnDataCb func(Msg)

// Source is the interface which is implemented by an audio source. This
// could be streaming data received from a network connection, a local
// audio source (e.g. microphone) or a local file.
type Source interface {
	Start() error
	Stop() error
	Close() error
	SetCb(OnDataCb)
}

// Sink is the interface which is implemented by an audio sink. This could
// be an Audio player or a file for recording.
type Sink interface {
	Start() error
	Stop() error
	Close() error
	SetVolume(float32)
	Volume() float32
	Write(Msg) error
	Flush()
}

// Token contains a sync.Waitgroup and is used with an audio sink. The
// token will indicate the application to wait until further audio buffers
// can be enqueued into the sink.
type Token struct {
	*sync.WaitGroup
}

// Msg contains an audio buffer with it's metadata.
type Msg struct {
	Data       []float32
	Samplerate float64
	Channels   int
	Frames     int  // Number of Frames in the buffer
	EOF        bool // End of File
}
