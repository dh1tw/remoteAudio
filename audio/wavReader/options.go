package wavReader

const (
	DefaultFramesPerBuffer int = 4096
)

// Option is the type for a function option
type Option func(*Options)

// Options contains the parameters for initializing a wav Reader.
type Options struct {
	FramesPerBuffer int
}

// FramesPerBuffer is a functional option which sets the amount of audio frames
// the wavReader will provide when executing the callback.
// Example: A buffer with 960 frames at 48000kHz / stereo contains
// 1920 samples and results in 20ms Audio.
func FramesPerBuffer(s int) Option {
	return func(args *Options) {
		args.FramesPerBuffer = s
	}
}
