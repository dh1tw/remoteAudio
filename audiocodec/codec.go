package audiocodec

type Encoder interface {
	Name() string
	Encode(interface{}, []byte) (int, error) // typically float32 input
}

type Decoder interface {
	Name() string
	Decode([]byte, []float32, ...Options) (int, error) //float32 output
}

type Option func(*Options)

type Options struct {
	Samplerate int
	Channels   int
	Bitdepth   int
}
