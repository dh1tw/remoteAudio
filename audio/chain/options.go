package chain

import "github.com/dh1tw/remoteAudio/audio"

// Option is the type for a function option
type Option func(*Options)

type Options struct {
	DefaultSource string
	DefaultSink   string
	Nodes         []audio.Node
}

func DefaultSource(s string) Option {
	return func(args *Options) {
		args.DefaultSource = s
	}
}

func DefaultSink(s string) Option {
	return func(args *Options) {
		args.DefaultSink = s
	}
}

func Node(n audio.Node) Option {
	return func(args *Options) {
		args.Nodes = append(args.Nodes, n)
	}
}
