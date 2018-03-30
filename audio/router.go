package audio

import (
	"fmt"
	"log"
	"sync"
)

// Router manages several audio sinks.
type Router interface {
	AddSink(string, Sink, bool) error
	RemoveSink(string) error
	Sink(string) (Sink, bool, error)
	// Sinks() map[string]Sink
	EnableSink(string, bool) error
	Write(Msg) SinkErrors
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
func (r *DefaultRouter) Write(msg Msg) SinkErrors {

	r.RLock()
	defer r.RUnlock()

	var sinkErrors []*SinkError

	for _, sink := range r.sinks {
		if !sink.active {
			continue
		}
		err := sink.Write(msg)
		// fmt.Println("writing to sink:", sinkName)
		// do smth with err!!!
		if err != nil {
			log.Println(err)
			// TBD: remove source (?)
			sErr := &SinkError{
				Sink:  sink,
				Error: err,
			}
			sinkErrors = append(sinkErrors, sErr)
		}
	}

	return sinkErrors
}

// AddSink adds an audio device which satisfies the Sink interface. When marked
// as active, incoming audio Msgs will be written to this device.
func (r *DefaultRouter) AddSink(name string, s Sink, active bool) error {
	r.Lock()
	defer r.Unlock()
	r.sinks[name] = &sink{s, active}
	if active {
		return s.Start()
	}

	return nil
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

// Sink returns the requested audio sink from the router. The boolean
// return parameter indicates if the sink is currently active.
func (r *DefaultRouter) Sink(name string) (Sink, bool, error) {
	r.RLock()
	defer r.RUnlock()
	s, ok := r.sinks[name]
	if !ok {
		return nil, false, fmt.Errorf("unknown sink %s", name)
	}
	return s, s.active, nil
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
		s.Flush()
		if s.active {
			// fmt.Println("flushing", sinkName)
		}
	}
}

// SinkError is an Error which is used when data could not be written to
// a particular audio Sink.
type SinkError struct {
	Sink  Sink
	Error error
}

type SinkErrors []*SinkError
