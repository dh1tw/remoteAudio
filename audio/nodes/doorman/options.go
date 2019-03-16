package doorman

// Option is the type for a function option
type Option func(*Options)

// Options contains the parameters for the Doorman
type Options struct {
	txUserChangedCb func(string)
}

// TXUserChanged is a functional option to provide a callback which get's
// called whenever the transmitting user changes.
func TXUserChanged(f func(string)) Option {
	return func(args *Options) {
		args.txUserChangedCb = f
	}
}
