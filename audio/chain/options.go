package chain

// Option is the type for a function option
type Option func(*Options)

type Options struct {
	DefaultSource string
	DefaultSink   string
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
