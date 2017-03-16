package event

import "sync"

type Event interface {
	Name() string
}

type Receiver interface {
	Receive(e Event)
}

type Publisher interface {
	Register(name string, p Receiver)

	Unregister(name string, p Receiver)

	Publish(e Event)
}

type simpleDispatcher struct {
	sync.Mutex

	registered map[string][]Receiver
}

func NewSimpleDispatcher() *simpleDispatcher {
	return &simpleDispatcher{
		registered: make(map[string][]Receiver),
	}
}

func (s *simpleDispatcher) Register(e string, p Receiver) {
	s.Lock()
	defer s.Unlock()

	for _, pp := range s.registered[e] {
		if pp == p {
			return
		}
	}
	s.registered[e] = append(s.registered[e], p)
}

func (s *simpleDispatcher) Unregister(e string, p Receiver) {
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
			break
		}
	}
	if idx == -1 {
		return
	}

	arr[idx] = arr[len(arr)-1]
	arr[len(arr)-1] = nil
	s.registered[e] = arr[:len(arr)-1]
}

func (s *simpleDispatcher) Publish(e Event) {
	s.Lock()
	defer s.Unlock()

	for _, p := range s.registered[e.Name()] {
		p.Receive(e)
	}
}
