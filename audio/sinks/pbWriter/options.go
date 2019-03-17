package pbWriter

import (
	"github.com/dh1tw/remoteAudio/audiocodec"
)

// Option is the type for a function option
type Option func(*Options)

// Options is the data structure holding the optional values. The
// values are typcially set by calling the functional options.
type Options struct {
	DeviceName      string
	Encoder         audiocodec.Encoder
	Channels        int
	Samplerate      float64
	FramesPerBuffer int
	UserID          string
	ToWireCb        func([]byte)
}

// Channels is a functional option to set the amount of channels to be used
// with the audio device. Typically this is either Mono (1) or Stereo (2).
// Make sure that your audio device supports the specified amount of channels.
func Channels(chs int) Option {
	return func(args *Options) {
		args.Channels = chs
	}
}

// Samplerate is a functional option to set the sampling rate of the
// audio device. Make sure your audio device supports the specified sampling
// rate.
func Samplerate(s float64) Option {
	return func(args *Options) {
		args.Samplerate = s
	}
}

// FramesPerBuffer is a functional option which sets the amount of sample frames
// our audio device will request / provide when executing the callback.
// Example: A buffer with 960 frames at 48000kHz / stereo contains
// 1920 samples and results in 20ms Audio.
func FramesPerBuffer(s int) Option {
	return func(args *Options) {
		args.FramesPerBuffer = s
	}
}

// Encoder is a functional option to set a specific decoder. By default,
// the opus decoder is used.
func Encoder(enc audiocodec.Encoder) Option {
	return func(args *Options) {
		args.Encoder = enc
	}
}

// UserID is a functional option to set the UserID. All serialized protobufs
// contain the UserID to mark their origin.
func UserID(s string) Option {
	return func(args *Options) {
		args.UserID = s
	}
}

// ToWireCb is a functional option to set the callback which will be executed
// when data has been serialized and is ready to be send to the network.
func ToWireCb(cb func([]byte)) Option {
	return func(args *Options) {
		args.ToWireCb = cb
	}
}
