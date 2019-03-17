package audiocodec

// Encoder is the interface which an audio encoder has to implement.
type Encoder interface {
	Name() string
	Encode(interface{}, []byte) (int, error) // typically float32 input
}

// Decoder is the interface which an audio decoder has to implement.
type Decoder interface {
	Name() string
	Decode([]byte, []float32, ...Options) (int, error) //float32 output
}

// Options is a struct which can be provided to the Decoder for particular
// Audio samples. This is useful in case default values shall be overwritten.
// However the decoder implementation has to support it, of course.
type Options struct {
	Samplerate int
	Channels   int
	Bitdepth   int
}
