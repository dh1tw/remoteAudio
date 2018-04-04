package audio

import (
	"fmt"
	"log"
	"sync"
)

// Router manages several audio sinks. It allows to write incoming
// audio Messages to serveral sinks (e.g. speakers, files, network..etc).
type Router interface {
	AddSink(string, Sink, bool) error
	RemoveSink(string) error
	Sink(string) (Sink, bool, error)
	EnableSink(string, bool) error
	Write(Msg) SinkErrors
	Close()
	Flush()
}

// SinkError is an Error which is used when data could not be written to
// a particular audio Sink.
type SinkError struct {
	Sink  Sink
	Error error
}

// SinkErrors is a convenience type which represents a slice of SinkError.
type SinkErrors []*SinkError

// DefaultRouter is the standard manager for audio sinks.
type DefaultRouter struct {
	sync.RWMutex // for map & variables
	sinks        map[string]*sink
}

type sink struct {
	Sink
	enabled bool
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
		if !sink.enabled {
			continue
		}
		err := sink.Write(msg)
		if err != nil {
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
// as enabled, incoming audio Msgs will be written to this device.
func (r *DefaultRouter) AddSink(name string, s Sink, enabled bool) error {
	r.Lock()
	defer r.Unlock()
	r.sinks[name] = &sink{s, enabled}
	if enabled {
		return s.Start()
	}

	return nil
}

// RemoveSink removes an audio sink from the router.
func (r *DefaultRouter) RemoveSink(name string) error {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.sinks[name]; !ok {
		return fmt.Errorf("unknown sink %s", name)
	}
	delete(r.sinks, name)
	return nil
}

// Sink returns the audio sink object. The boolean return value indicates
// if the sink is currently enabled. If no sink is found under the specified
// name, an error will be returned.
func (r *DefaultRouter) Sink(name string) (Sink, bool, error) {
	r.RLock()
	defer r.RUnlock()
	s, ok := r.sinks[name]
	if !ok {
		return nil, false, fmt.Errorf("unknown sink %s", name)
	}
	return s, s.enabled, nil
}

// EnableSink will mark the audio Sink as enabled, so that incoming audio
// Msgs will be written to it.
func (r *DefaultRouter) EnableSink(name string, enabled bool) error {
	r.Lock()
	defer r.Unlock()
	s, ok := r.sinks[name]
	if !ok {
		return fmt.Errorf("unknown sink %s", name)
	}
	s.enabled = enabled
	if s.enabled {
		return s.Start()
	}
	return s.Stop()
}

// Flush flushes the buffers of all enabled sinks.
func (r *DefaultRouter) Flush() {
	r.RLock()
	defer r.RUnlock()
	for _, s := range r.sinks {
		s.Flush()
	}
}

// Close disables and closes all the Sinks of the router.
func (r *DefaultRouter) Close() {
	r.Lock()
	defer r.Unlock()

	for _, sink := range r.sinks {
		sink.Flush()
		if err := sink.Stop(); err != nil {
			log.Println(err)
		}
		if err := sink.Close(); err != nil {
			log.Println(err)
		}
	}
}
