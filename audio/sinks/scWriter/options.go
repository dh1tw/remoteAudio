package scWriter

import "time"

// Option is the type for a function option
type Option func(*Options)

// Options contains the parameters for initializing a sound card writer.
type Options struct {
	HostAPI         string
	DeviceName      string
	Channels        int
	Samplerate      float64
	FramesPerBuffer int
	Latency         time.Duration
	RingBufferSize  int
}

// HostAPI is a functional option to enforce the usage of a particular
// audio host API
func HostAPI(hostAPI string) Option {
	return func(args *Options) {
		args.HostAPI = hostAPI
	}
}

// DeviceName is a functional option to specify the name of the
// Audio device
func DeviceName(name string) Option {
	return func(args *Options) {
		args.DeviceName = name
	}
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

// Latency is a functional option to set the latency of the audio device.
func Latency(t time.Duration) Option {
	return func(args *Options) {
		args.Latency = t
	}
}

// RingBufferSize is a functional option to set the size of the ring buffer
// of Output audio devices. When enqueing samples, they are stored in the ring
// buffer from where the callback will retrieve them for playing them on the
// speaker.
func RingBufferSize(size int) Option {
	return func(args *Options) {
		args.RingBufferSize = size
	}
}
