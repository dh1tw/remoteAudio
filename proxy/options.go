package proxy

// Option is the type for a function option
type Option func(*Options)

type Options struct {
}

// Channels is a functional option to set the amount of channels to be used
// with the audio device. Typically this is either Mono (1) or Stereo (2).
// Make sure that your audio device supports the specified amount of channels.

// func Channels(chs int) Option {
// 	return func(args *Options) {
// 		args.Channels = chs
// 	}
// }
