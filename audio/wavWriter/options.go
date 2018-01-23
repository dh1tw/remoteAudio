package wavWriter

// Option is the type for a function option
type Option func(*Options)

const (
	DefaultChannels   int     = 1
	DefaultSamplerate float64 = 48000
	DefaultBitDepth   int     = 16
)

// Options contains the parameters for initializing a wav writer.
type Options struct {
	Channels   int
	Samplerate float64
	BitDepth   int
}

// Channels is a functional option to set the amount of channels to be used
// with the audio device. Typically this is either Mono (1) or Stereo (2).
func Channels(chs int) Option {
	return func(args *Options) {
		args.Channels = chs
	}
}

// Samplerate is a functional option to set the sampling rate with which the
// audio will be recorded. The higher the samplerate, the higher larger the
// recordings are.
func Samplerate(s float64) Option {
	return func(args *Options) {
		args.Samplerate = s
	}
}

// BitDepth is a functional option to set the bit depth with which the audio
// will be written to file. The Bitdepth (12/16/32 bit) defines the dynamic range
// of the audio. For most usecases 16 bit (default) is the way to go. The
// higher the bitDepth, the larger the recordings are.
func BitDepth(b int) Option {
	return func(args *Options) {
		args.BitDepth = b
	}
}
