package network

import (
	"fmt"
	"sync"
	"time"

	"github.com/dedis/onet/log"
	"github.com/nikkolasg/mulsigo/event"
)

// Router handles all networking operations such as:
//   * listening to incoming connections using a host.Listener method
//   * opening up new connections using host.Connect method
//   * dispatching incoming message using a Dispatcher
//   * dispatching outgoing message maintaining a translation
//   between ServerIdentity <-> address
//   * managing the re-connections of non-working Conn
// Most caller should use the creation function like NewTCPRouter(...),
// NewLocalRouter(...) then use the Host such as:
//
//   router.Start() // will listen for incoming Conn and block
//   router.Stop() // will stop the listening and the managing of all Conn
type Router struct {
	// address is the real-actual address used by the listener.
	address Address
	// Dispatcher is used to dispatch incoming message to the right recipient
	Dispatcher
	// Host listens for new connections
	host Host
	// connections keeps track of all active connections. Because a connection
	// can be opened at the same time on both endpoints, there can be more
	// than one connection per ServerIdentityID.
	connections map[Address]Conn
	sync.Mutex

	// boolean flag indicating that the router is already clos{ing,ed}.
	isClosed bool

	// wg waits for all handleConn routines to be done.
	wg sync.WaitGroup

	// keep bandwidth of closed connections
	traffic counterSafe

	event.Publisher
}

// NewRouter returns a new Router attached to a ServerIdentity and the host we want to
// use.
func NewRouter(h Host) *Router {
	r := &Router{
		connections: make(map[Address]Conn),
		host:        h,
		Dispatcher:  NewBlockingDispatcher(),
		Publisher:   event.NewSimpleDispatcher(),
	}
	r.address = h.Address()
	return r
}

// Start the listening routine of the underlying Host. This is a
// blocking call until r.Stop() is called.
func (r *Router) Start() {
	// Any incoming connection waits for the remote server identity
	// and will create a new handling routine.
	err := r.host.Listen(func(c Conn) {
		if err := r.registerConnection(c); err != nil {
			log.Lvlf3("router %s: can't register conn to %s", r.address, c.Remote())
			return
		}
		// start handleConn in a go routine that waits for incoming messages and
		// dispatches them.
		if err := r.launchHandleRoutine(c); err != nil {
			log.Lvlf3("router %s: can't launch routine for %s", r.address, c.Remote())
			return
		}
	})
	if err != nil {
		log.Error("router: listening:", err)
	}
}

// Stop the listening routine, and stop any routine of handling
// connections. Calling r.Start(), then r.Stop() then r.Start() again leads to
// an undefined behaviour. Callers should most of the time re-create a fresh
// Router.
func (r *Router) Stop() error {
	var err error
	err = r.host.Stop()
	r.Lock()
	// set the isClosed to true
	r.isClosed = true

	// then close all connections
	for _, c := range r.connections {
		if err := c.Close(); err != nil {
			log.Lvl5(err)
		}
		c.Close()
	}
	// wait for all handleConn to finish
	r.Unlock()
	r.wg.Wait()

	if err != nil {
		return err
	}
	return nil
}

// Send sends to an ServerIdentity without wrapping the msg into a ProtocolMsg
func (r *Router) Send(to Address, msg Message) error {
	if msg == nil {
		return fmt.Errorf("router %s: can't send nil-packet", r.address)
	}

	c := r.connection(to)
	if c == nil {
		var err error
		c, err = r.connect(to)
		if err != nil {
			return err
		}
	}

	log.Lvlf4("router %s: sends to %s msg: %+v", r.address, to, msg)
	var err error
	err = c.Send(msg)
	if err != nil {
		log.Lvlf3("router %s: couldn't send to %s (trying again): %s", r.address, to, err)
		c, err := r.connect(to)
		if err != nil {
			return err
		}
		err = c.Send(msg)
		if err != nil {
			return err
		}
	}
	return nil
}

// connect starts a new connection and launches the listener for incoming
// messages.
func (r *Router) connect(addr Address) (Conn, error) {
	log.Lvlf3("router %s: connecting to %s", r.address, addr)
	c, err := r.host.Connect(addr)
	if err != nil {
		log.Lvlf3("router %s: could not connect to %s: %s", r.address, addr, err)
		return nil, err
	}
	log.Lvlf3("router %s: connected to %s", r.address, addr)
	if err := r.registerConnection(c); err != nil {
		return nil, err
	}

	if err := r.launchHandleRoutine(c); err != nil {
		return nil, err
	}
	return c, nil

}

func (r *Router) removeConnection(c Conn) {
	r.Lock()
	defer r.Unlock()

	delete(r.connections, c.Remote())
}

// handleConn waits for incoming messages and calls the dispatcher for
// each new message. It only quits if the connection is closed or another
// unrecoverable error in the connection appears.
func (r *Router) handleConn(c Conn) {
	addr := c.Remote()
	defer func() {
		// Clean up the connection by making sure it's closed.
		if err := c.Close(); err != nil {
			log.Lvlf5("router %s: error closing conn to %s: %s", r.address, addr, err)
		}
		log.LLvlf5("router %s: closing conn to %s", r.address, addr)
		r.traffic.updateRx(c.Rx())
		r.traffic.updateTx(c.Tx())
		log.LLvlf5("router %s: closing conn to %s #1", r.address, addr)
		r.wg.Done()
		log.LLvlf5("router %s: closing conn to %s #2", r.address, addr)
		r.removeConnection(c)
		log.LLvlf5("router %s: closing conn to %s #3", r.address, addr)
		//r.Publish(&EventDown{c.Remote()})
		log.LLvlf5("router %s: closing conn to %s #4", r.address, addr)
	}()
	log.Lvlf3("router %s: handling new connection to %s", r.address, addr)
	for {
		fmt.Println("TCPCON ", c.Remote(), " --> Receive()")
		packet, err := c.Receive()
		fmt.Println("TCPCON ", c.Remote(), " --> Receive() DONE", err)
		if r.Closed() {
			return
		}

		fmt.Println("TCPCON ", c.Remote(), " --> After r.Closed()")
		if err != nil {
			if err == ErrTimeout {
				log.Lvlf5("router %s: drops connection to %s: timeout", r.address, addr)
				return
			}

			if err == ErrClosed || err == ErrEOF {
				// Connection got closed.
				log.Lvlf5("router %s: drops connection %s: closed", r.address, addr)
				return
			}
			// Temporary error, continue.
			log.Lvlf3("router %s: error with connection %s", r.address, addr)
			continue
		}

		fmt.Println("TCPCON ", c.Remote(), " --> before r.Dispatch()")
		if err := r.Dispatch(addr, packet); err != nil {
			log.Lvlf3("router %s: error dispatching %s", r.address, err)
		}

		fmt.Println("TCPCON ", c.Remote(), " --> After Dispatch()")
	}
}

// connection returns the first connection associated with this ServerIdentity.
// If no connection is found, it returns nil.
func (r *Router) connection(addr Address) Conn {
	r.Lock()
	defer r.Unlock()
	return r.connections[addr]
}

// registerConnection registers a ServerIdentity for a new connection, mapped with the
// real physical address of the connection and the connection itself.
// It uses the networkLock mutex.
func (r *Router) registerConnection(c Conn) error {
	log.Lvlf4("router %s: registers conn to %s", r.address, c.Remote())
	r.Lock()
	defer r.Unlock()
	if r.isClosed {
		return ErrClosed
	}
	_, okc := r.connections[c.Remote()]
	if okc {
		log.Lvlf5("router %s: connection to %s already registered.", r.address, c.Remote())
		return nil
	}
	r.connections[c.Remote()] = c
	r.Publish(&EventUp{c.Remote(), c})
	return nil
}

func (r *Router) launchHandleRoutine(c Conn) error {
	r.Lock()
	defer r.Unlock()
	if r.isClosed {
		return ErrClosed
	}
	r.wg.Add(1)
	go r.handleConn(c)
	// XXX weird hack for test TestRouterLotsOfConnLocal
	time.Sleep(100 * time.Microsecond)
	return nil
}

// Closed returns true if the router is closed (or is closing). For a router
// to be closed means that a call to Stop() must have been made.
func (r *Router) Closed() bool {
	r.Lock()
	defer r.Unlock()
	return r.isClosed
}

// Tx implements monitor/CounterIO
// It returns the Tx for all connections managed by this router
func (r *Router) Tx() uint64 {
	r.Lock()
	defer r.Unlock()
	var tx uint64
	for _, c := range r.connections {
		tx += c.Tx()
	}
	tx += r.traffic.Tx()
	return tx
}

// Rx implements monitor/CounterIO
// It returns the Rx for all connections managed by this router
func (r *Router) Rx() uint64 {
	r.Lock()
	defer r.Unlock()
	var rx uint64
	for _, c := range r.connections {
		rx += c.Rx()
	}
	rx += r.traffic.Rx()
	return rx
}

// Listening returns true if this router is started.
func (r *Router) Listening() bool {
	return r.host.Listening()
}

const EventConnUp = "EventConnUp"
const EventConnDown = "EventConnDown"

type EventUp struct {
	Address Address
	Conn    Conn
}

func (e *EventUp) Name() string {
	return EventConnUp
}

type EventDown struct {
	Address Address
}

func (e *EventDown) Name() string {
	return EventConnDown
}
