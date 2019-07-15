package vox

import "time"

// Option is the type for a function option
type Option func(*Vox)

// Enabled is a functional option to initialize the Vox object
// with an enabled or disabled vox.
func Enabled(enabled bool) Option {
	return func(v *Vox) {
		v.enabled = enabled
	}
}

// StateChanged is a functional option to provide a callback which be
// executed whenever the Vox is triggered or cut off.
func StateChanged(f func(bool)) Option {
	return func(v *Vox) {
		v.onStateChange = f
	}
}

// Threshold is a functional option to set the initial audio
// threshold level when the vox kick's in. The range must be
// between 0 ... 1.
func Threshold(t float32) Option {
	return func(v *Vox) {
		v.threshold = t
	}
}

// HoldTime is a function option to set the hold time of the vox.
func HoldTime(t time.Duration) Option {
	return func(v *Vox) {
		v.holdTime = t
	}
}
