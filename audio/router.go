package audio

import (
	"fmt"
	"sync"
)

// Router manages several audio sinks.
type Router interface {
	AddSink(string, Sink, bool)
	RemoveSink(string) error
	Sink(string) (Sink, error)
	// Sinks() map[string]Sink
	EnableSink(string, bool) error
	Write(Msg) WriteToken
	Flush()
}

type sink struct {
	Sink
	active bool
}

// DefaultRouter is the standard manager for audio sinks.
type DefaultRouter struct {
	sync.RWMutex // for map & variables
	sinks        map[string]*sink
}

// NewDefaultRouter returns an initialized default router for audio sinks.
func NewDefaultRouter() (*DefaultRouter, error) {

	r := &DefaultRouter{
		sinks: make(map[string]*sink),
	}

	return r, nil
}

// Write will write the Msg to all enabled audio sinks.
func (r *DefaultRouter) Write(msg Msg) WriteToken {

	token := Token{&sync.WaitGroup{}}

	r.RLock()
	defer r.RUnlock()

	var sinkErrors []*SinkError

	for _, sink := range r.sinks {
		if !sink.active {
			continue
		}
		err := sink.Write(msg, token)
		// do smth with err!!!
		if err != nil {
			// TBD: remove source (?)
			sErr := &SinkError{
				Sink:  sink,
				Error: err,
			}
			sinkErrors = append(sinkErrors, sErr)
		}
	}

	return WriteToken{token, sinkErrors}
}

// AddSink adds an audio device which satisfies the Sink interface. When marked
// as active, incoming audio Msgs will be written to this device.
func (r *DefaultRouter) AddSink(name string, s Sink, active bool) {
	r.Lock()
	defer r.Unlock()
	r.sinks[name] = &sink{s, active}
}

// RemoveSink removes an audio sink.
func (r *DefaultRouter) RemoveSink(name string) error {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.sinks[name]; !ok {
		return fmt.Errorf("unknown sink %s", name)
	}
	delete(r.sinks, name)
	return nil
}

// Sink returns the requested audio Sink from the router.
func (r *DefaultRouter) Sink(name string) (Sink, error) {
	r.RLock()
	defer r.RUnlock()
	s, ok := r.sinks[name]
	if !ok {
		return nil, fmt.Errorf("unknown sink %s", name)
	}
	return s, nil
}

// func (r *DefaultRouter) Sinks() map[string]Sink {
// 	r.RLock()
// 	defer r.RUnlock()
// 	return r.sinks // concurrency? deep copy or shallow copy?
// }

// EnableSink will mark the audio Sink as active, so that incoming audio
// Msgs will be written to it.
func (r *DefaultRouter) EnableSink(name string, active bool) error {
	r.Lock()
	defer r.Unlock()
	s, ok := r.sinks[name]
	if !ok {
		return fmt.Errorf("unknown sink %s", name)
	}
	s.active = active
	if s.active {
		return s.Start()
	}
	return s.Stop()
}

// Flush flushes the buffers of all active sinks.
func (r *DefaultRouter) Flush() {
	r.RLock()
	defer r.RUnlock()
	for _, s := range r.sinks {
		if s.active {
			s.Flush()
		}
	}
}

// WriteToken contains a sync.Waitgroup and is used with an audio sink. The
// token will indicate the application to wait until further audio buffers
// can be enqueued into the sink. In case writing to one or more sources
// resulted in an error, err will be != nil.
type WriteToken struct {
	Token
	Error []*SinkError
}

// SinkError is an Error which is used when data could not be written to
// a particular audio Sink.
type SinkError struct {
	Sink  Sink
	Error error
}
