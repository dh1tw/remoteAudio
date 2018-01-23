package audio

import (
	"fmt"
	"sync"
)

type Selector interface {
	AddSource(string, Source)
	RemoveSource(string) error
	SetSource(string) error
	SetCb(OnDataCb)
}

type selector struct {
	sync.Mutex
	sources map[string]*source
	cb      OnDataCb
}

type source struct {
	Source
	active bool
}

func NewSelector() (Selector, error) {

	s := &selector{
		sources: make(map[string]*source),
	}

	return s, nil
}

func (s *selector) AddSource(name string, src Source) {
	s.Lock()
	defer s.Unlock()
	s.sources[name] = &source{src, false}
}

func (s *selector) RemoveSource(name string) error {
	s.Lock()
	defer s.Unlock()

	if _, ok := s.sources[name]; !ok {
		return fmt.Errorf("unknown source %s", name)
	}
	delete(s.sources, name)
	return nil
}

func (s *selector) SetSource(name string) error {
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

func (s *selector) SetCb(cb OnDataCb) {
	s.cb = cb
}
