package client

import (
	"crypto/sha256"
	"strings"
	"sync"

	"github.com/nikkolasg/mulsigo/slog"
)

// Dispatcher receives messages, decode them and dispatch them to the registered
// processors. If multiple processors are registered, each message is dispatched
// to each processors.
type Dispatcher interface {
	RegisterProcessor(Processor)
	Dispatch(*Identity, *ClientMessage)
}

// Processor receives a message from a dispatcher.
type Processor interface {
	Process(*Identity, *ClientMessage)
}

// Router is the main "network" entry point enabling one to send messages to a
// peer and to receive any messages.
// If it were an interface, it would look like:
//   type Router interface
//      Dispatcher
//   	Send(*Identity, *ClientMessage) error
//   	Broadcast(cm *ClientMessage, ids ...*Identity) error
//   	Close()
//   }
//
// Originally, the router was an interface but then extracted the only generic
// behavior to streamFactory.
type Router struct {
	// dispatcher used to dispatch message
	Dispatcher
	// factory used to instantiate new streams
	streamFactory
	// global lock making one Send at a time
	// XXX would there be a way to remove that constraint...
	sync.Mutex
	// list of open streams maintained by the router
	streams map[string]stream
	doneMut sync.Mutex
	done    bool
}

// NewRouter returns a router that handles streams connections and is the main
// public înterface to send / receive message
func NewRouter(factory streamFactory) *Router {
	blr := &Router{
		streamFactory: factory,
		streams:       make(map[string]stream),
		Dispatcher:    newSeqDispatcher(),
	}
	return blr
}

// Send fetch or create the corresponding channel corresponding to the pair tied
// to the given remote identity. It then sends the message down that channel.
func (blr *Router) Send(remote *Identity, msg *ClientMessage) error {
	blr.Lock()
	defer blr.Unlock()
	if blr.done {
		return ErrClosed
	}
	stream, ok := blr.streams[remote.ID()]
	if !ok {
		var err error
		stream, err = blr.newStream(remote)
		if err != nil {
			return err
		}
		go blr.processStream(remote, stream)
	}
	buf, err := enc.Marshal(msg)
	if err != nil {
		return err
	}
	return stream.send(buf)
}

func (blr *Router) processStream(id *Identity, s stream) {
	for {
		buff, err := s.receive()
		if err != nil {
			slog.Info("relay router: closing stream with ", id.Name)
			return
		}
		unmarshald, err := enc.Unmarshal(buff)
		if err != nil {
			slog.Info("relay router: error unmarshalling:", err)
			return
		}
		msg := unmarshald.(*ClientMessage)
		blr.Dispatch(id, msg)
	}
}

// Broadcast sends the message in parallel to all destinations given in ids. It
// returns the first error encountered.
func (blr *Router) Broadcast(msg *ClientMessage, ids ...*Identity) error {
	var done = make(chan error)
	for _, i := range ids {
		go func() {
			err := blr.Send(i, msg)
			done <- err
		}()
	}

	n := 0
	for n < len(ids) {
		err := <-done
		if err != nil {
			return err
		}
	}
	return nil
}

// Close will close all registered streams.
func (blr *Router) Close() {
	blr.Lock()
	defer blr.Unlock()
	for _, s := range blr.streams {
		s.close()
	}
	blr.done = true
}

// channelID returns the channel id associated with the two given identity. It's
// basically base64-encoded and sorted, then hashed. The second return value
// denotes if own is first or not (useful to designate initiator).
func channelID(own, remote *Identity) (string, bool) {
	str1 := own.ID()
	str2 := remote.ID()
	comp := strings.Compare(str1, str2)
	var s1, s2 string
	var first bool
	if comp < 0 {
		s1 = str2[:16]
		s2 = str1[16:]
		first = true
	} else if comp > 0 {
		s1 = str1[:16]
		s2 = str2[16:]
	}
	h := sha256.New()
	h.Write([]byte(s1))
	h.Write([]byte(s2))
	return string(h.Sum(nil)), first
}

// seqDispatcher is a simple dispatcher sequentially dispatching message to the
// registered processors.
type seqDispatcher struct {
	procs []Processor
}

// newSeqDispatcher returns a Dispatcher that dispatch messages sequentially to
// all registered processors, in the same go routine.
func newSeqDispatcher() Dispatcher {
	return &seqDispatcher{}
}

// RegisterProcessor implements the Dispatcher interface.
func (s *seqDispatcher) RegisterProcessor(p Processor) {
	s.procs = append(s.procs, p)
}

// Dispatch implements the Dispatcher interface.
func (s *seqDispatcher) Dispatch(i *Identity, cm *ClientMessage) {
	for _, p := range s.procs {
		p.Process(i, cm)
	}
}

// NonBlockingRouter is a middleware around Router that sends and broadcasts
// messages in a asynchornous way using goroutine. Each methods returns a
// channel that will dispatch any future error if any.
type NonBlockingRouter struct {
	Router
}

// NewNonBlockingRouter returns a router that sends message asynchronously.
func NewNonBlockingRouter(r Router) *NonBlockingRouter {
	return &NonBlockingRouter{r}
}

// Send use the underlying router's Send method to send the message in a
// goroutine. It returns a channel where the error will be dispatched.
func (n *NonBlockingRouter) Send(id *Identity, cm *ClientMessage) chan error {
	var e = make(chan error)
	go func() { err := n.Router.Send(id, cm); e <- err }()
	return e
}

func (n *NonBlockingRouter) Broadcast(cm *ClientMessage, ids ...*Identity) chan error {
	panic("not implemented yet")
}
