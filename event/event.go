package event

import "sync"

type Event interface {
	Name() string
}

type Processor interface {
	Process(e Event)
}

type Dispatcher interface {
	Register(name string, p Processor)

	Unregister(name string, p Processor)

	Dispatch(e Event)
}

type simpleDispatcher struct {
	sync.Mutex

	registered map[string][]Processor
}

func NewSimpleDispatcher() *simpleDispatcher {
	return &simpleDispatcher{
		registered: make(map[string][]Processor),
	}
}

func (s *simpleDispatcher) Register(e string, p Processor) {
	s.Lock()
	defer s.Unlock()

	for _, pp := range s.registered[e] {
		if pp == p {
			return
		}
	}
	s.registered[e] = append(s.registered[e], p)
}

func (s *simpleDispatcher) Unregister(e string, p Processor) {
	s.Lock()
	defer s.Unlock()

	var idx = -1
	arr, ok := s.registered[e]
	if !ok {
		return
	}
	for i, pp := range arr {
		if pp == p {
			idx = i
		}
	}
	if idx == -1 {
		return
	}

	arr[idx] = arr[len(arr)-1]
	arr[len(arr)-1] = nil
	s.registered[e] = arr[:len(arr)-1]
}

func (s *simpleDispatcher) Dispatch(e Event) {
	s.Lock()
	defer s.Unlock()

	for _, p := range s.registered[e.Name()] {
		p.Process(e)
	}
}
