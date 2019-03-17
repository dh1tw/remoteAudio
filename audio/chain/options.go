package chain

import "github.com/dh1tw/remoteAudio/audio"

// Option is the type for a function option
type Option func(*Options)

// Options is a data structure which holds the option values. The values
// are set through functional options.
type Options struct {
	DefaultSource string
	DefaultSink   string
	Nodes         []audio.Node
}

// DefaultSource is a functional option which sets the name of the default source
// of an audio chain.
func DefaultSource(s string) Option {
	return func(args *Options) {
		args.DefaultSource = s
	}
}

// DefaultSink is a functional option which sets the name of the default sink
// of an audio chain.
func DefaultSink(s string) Option {
	return func(args *Options) {
		args.DefaultSink = s
	}
}

// Node is a functional option which adds a particular audio.Node to the
// audio chain. The Node will be put between a the Source and Sink. If serveral
// Nodes are provided, the order matters. The last Node will be connected
// to the source, the first one to the sink.
func Node(n audio.Node) Option {
	return func(args *Options) {
		args.Nodes = append(args.Nodes, n)
	}
}
