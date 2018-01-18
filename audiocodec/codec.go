package audiocodec

type Audiocodec interface {
	Name() string
	Options() Options
}

type Encoder interface {
	Audiocodec
	Encode(interface{}) ([]byte, error) // typically float32 input
}

type Decoder interface {
	Audiocodec
	Decode(interface{}, interface{}, ...Options) error //float32 output
}

type Option func(*Option)

type Options struct {
	Samplingrate int
	Channels     int
	Bitdepth     int
}
