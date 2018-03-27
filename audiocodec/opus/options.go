package opus

import opus "gopkg.in/hraban/opus.v2"

type Option func(*Options)

type Options struct {
	Name         string
	Samplerate   float64
	Channels     int
	Bitrate      int
	MaxBandwidth opus.Bandwidth
	Application  opus.Application
	Complexity   int
}

// Channels is a functional option to set the amount of channels to be used
// with the audio device. Typically this is either Mono (1) or Stereo (2).
// Make sure that your audio device supports the specified amount of channels.
// By default the encoder uses 1 channel, and the decoder 2 channels.
func Channels(chs int) Option {
	return func(args *Options) {
		args.Channels = chs
	}
}

// Samplerate is a functional option to set the sampling rate of the
// audio device. Make sure your audio device supports the specified sampling
// rate. By default, the encoder is set to 48kHz.
func Samplerate(s float64) Option {
	return func(args *Options) {
		args.Samplerate = s
	}
}

// Application is a functional option through which the Encoder profile
// can be selected. By default, RestrictedLowdelay is set. Check the opus
// documentation to learn more about the different profiles.
func Application(a int) Option {
	return func(args *Options) {
		args.Application = opus.Application(a)
	}
}

// MaxBandwidth is a functional option to set the maximum bandpass that the
// encoder can select. Check the opus documentation to learn more about the
// available bandwidths. By default Wideband (8kHz) is selected.
func MaxBandwidth(maxBw int) Option {
	return func(args *Options) {
		args.MaxBandwidth = opus.Bandwidth(maxBw)
	}
}

// Bitrate is a functional option to set the output bitrate of the
// opus encoder. The default value is 24kbit/s.
func Bitrate(rate int) Option {
	return func(args *Options) {
		args.Bitrate = rate
	}
}

// Complexity is a functional option to set the compuational complexity
// of the opus encoder. For the specific values, check the opus encoder
// documentation. By default a complexity of 5 is selected.
func Complexity(c int) Option {
	return func(args *Options) {
		args.Complexity = c
	}
}
