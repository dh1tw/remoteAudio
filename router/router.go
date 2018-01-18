package router

import (
	"fmt"
	"sync"

	"github.com/dh1tw/remoteAudio/audio"
)

type Router interface {
	AddSink(string, audio.Sink, bool)
	RemoveSink(string) error
	Sink(string) (audio.Sink, error)
	// Sinks() map[string]audio.Sink
	EnableSink(string, bool) error
	Enqueue(audio.AudioMsg) audio.Token
	Flush()
}

type sink struct {
	audio.Sink
	active bool
}

type router struct {
	sync.RWMutex // for map & variables
	sinks        map[string]*sink
}

func NewRouter(opts ...Option) (Router, error) {

	r := &router{
		sinks: make(map[string]*sink),
	}

	return r, nil
}

func (r *router) Enqueue(msg audio.AudioMsg) audio.Token {

	token := audio.NewToken()

	r.RLock()
	defer r.RUnlock()

	for _, sink := range r.sinks {
		if !sink.active {
			continue
		}
		sink.Enqueue(msg, token)
	}

	return token
}

func (r *router) AddSink(name string, s audio.Sink, active bool) {
	r.Lock()
	defer r.Unlock()
	r.sinks[name] = &sink{s, active}
}

func (r *router) RemoveSink(name string) error {
	r.Lock()
	defer r.Unlock()
	if _, ok := r.sinks[name]; !ok {
		return fmt.Errorf("unknown sink %s", name)
	}
	delete(r.sinks, name)
	return nil
}

func (r *router) Sink(name string) (audio.Sink, error) {
	r.RLock()
	defer r.RUnlock()
	s, ok := r.sinks[name]
	if !ok {
		return nil, fmt.Errorf("unknown sink %s", name)
	}
	return s, nil
}

// func (r *router) Sinks() map[string]audio.Sink {
// 	r.RLock()
// 	defer r.RUnlock()
// 	return r.sinks // concurrency? deep copy or shallow copy?
// }

func (r *router) EnableSink(name string, active bool) error {
	r.Lock()
	defer r.Unlock()
	s, ok := r.sinks[name]
	if !ok {
		return fmt.Errorf("unknown sink %s", name)
	}
	s.active = active
	return nil
}

func (r *router) Flush() {
	r.RLock()
	defer r.RUnlock()
	for _, s := range r.sinks {
		if s.active {
			s.Flush()
		}
	}
}

type Options struct {
}

type Option func(*Options)
