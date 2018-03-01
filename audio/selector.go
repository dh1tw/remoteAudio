package audio

import (
	"fmt"
	"sync"
)

// Selector manages several audio sources.
type Selector interface {
	AddSource(string, Source)
	RemoveSource(string) error
	SetSource(string) error
	SetCb(OnDataCb)
}

// DefaultSelector is the default implementation of an audio Selector.
type DefaultSelector struct {
	sync.Mutex
	sources map[string]*source
	cb      OnDataCb
}

type source struct {
	Source
	active bool
}

// NewDefaultSelector returns an initialized, but empty DefaultSelector.
func NewDefaultSelector() (*DefaultSelector, error) {

	s := &DefaultSelector{
		sources: make(map[string]*source),
	}

	return s, nil
}

// AddSource adds an audio device which implements the audio.Source interface
// to the Selector.
func (s *DefaultSelector) AddSource(name string, src Source) {
	s.Lock()
	defer s.Unlock()
	s.sources[name] = &source{src, false}
}

// RemoveSource removes an audio Source from the Selector.
func (s *DefaultSelector) RemoveSource(name string) error {
	s.Lock()
	defer s.Unlock()

	if _, ok := s.sources[name]; !ok {
		return fmt.Errorf("unknown source %s", name)
	}
	delete(s.sources, name)
	return nil
}

// SetSource selects the active source.
func (s *DefaultSelector) SetSource(name string) error {
	s.Lock()
	defer s.Unlock()

	if s.cb == nil {
		return fmt.Errorf("selector callback not set")
	}

	if _, ok := s.sources[name]; !ok {
		return fmt.Errorf("unknown source %s", name)
	}

	for _, src := range s.sources {
		src.active = false
		src.Stop()
		src.SetCb(nil)
	}

	s.sources[name].active = true
	s.sources[name].SetCb(s.cb)
	s.sources[name].Start()

	return nil
}

// SetCb sets the callback function will will be executed when new audio
// msgs are available from the selected source.
func (s *DefaultSelector) SetCb(cb OnDataCb) {
	s.cb = cb
}
