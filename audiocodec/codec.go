package audiocodec

type Audiocodec interface {
	Name() string
	Options() Options
}

type Encoder interface {
	Audiocodec
	Encode(interface{}, []byte) (int, error) // typically float32 input
}

type Decoder interface {
	Audiocodec
	Decode([]byte, []float32, ...Options) (int, error) //float32 output
}

type Option func(*Options)

type Options struct {
	Samplerate int
	Channels   int
	Bitdepth   int
}
