package network

import (
	"sync"
	"testing"
	"time"

	"github.com/dedis/onet/log"
	"github.com/nikkolasg/mulsigo/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var basicMessageEncoder = NewSingleProtoEncoder(&basicMessage{})

func NewTestRouterTCP(port int) (*Router, error) {
	h, err := NewTestTCPHost(port)
	if err != nil {
		return nil, err
	}
	return NewRouter(h), nil
}

func NewTestRouterLocal(port int) (*Router, error) {
	h, err := NewTestLocalHost(port)
	if err != nil {
		return nil, err
	}
	return NewRouter(h), nil
}

type routerFactory func(port int) (*Router, error)

// Test if router fits the interface such as calling Run(), then Stop(),
// should return
func TestRouterTCP(t *testing.T) {
	testRouter(t, NewTestRouterTCP)
}
func TestRouterLocal(t *testing.T) {
	testRouter(t, NewTestRouterLocal)
}

func testRouter(t *testing.T, fac routerFactory) {
	h, err := fac(2004)
	if err != nil {
		t.Fatal(err)
	}
	var stop = make(chan bool)
	go func() {
		stop <- true
		h.Start()
		stop <- true
	}()
	<-stop
	// Time needed so the listener is up. Equivalent to "connecting ourself" as
	// we had before.
	time.Sleep(250 * time.Millisecond)
	h.Stop()
	select {
	case <-stop:
		return
	case <-time.After(500 * time.Millisecond):
		t.Fatal("TcpHost should have returned from Run() by now")
	}
}

func testRouterRemoveConnection(t *testing.T) {
	r1, err := NewTestRouterTCP(2008)
	require.Nil(t, err)
	r2, err := NewTestRouterTCP(2009)
	require.Nil(t, err)

	defer r1.Stop()

	go r1.Start()
	go r2.Start()

	require.NotNil(t, r1.Send(r2.address, nil))

	r1.Lock()
	require.NotNil(t, r1.connections[r2.address])
	r1.Unlock()

	require.Nil(t, r2.Stop())

	r1.Lock()
	require.Nil(t, r1.connections[r2.address])
	r1.Unlock()
}

// Test the automatic connection upon request
func TestRouterAutoConnectionTCP(t *testing.T) {
	testRouterAutoConnection(t, NewTestRouterTCP)
}
func TestRouterAutoConnectionLocal(t *testing.T) {
	testRouterAutoConnection(t, NewTestRouterLocal)
}

func testRouterAutoConnection(t *testing.T, fac routerFactory) {
	h1, err := fac(2007)
	if err != nil {
		t.Fatal(err)
	}
	err = h1.Send(NewLocalAddress("127.1.2.3:2890"), &SimpleMessage{12})
	if err == nil {
		t.Fatal("Should not be able to send")
	}
	h2, err := fac(2008)
	if err != nil {
		t.Fatal(err)
	}

	err = h1.Send(h2.address, nil)
	require.NotNil(t, err)

	go h2.Start()
	for !h2.Listening() {
		time.Sleep(10 * time.Millisecond)
	}

	clean := func() {
		assert.Nil(t, h1.Stop())
		assert.Nil(t, h2.Stop())
	}
	defer clean()

	proc := newSimpleMessageProc(t)
	h2.RegisterProcessor(proc, &SimpleMessage{})
	h1.RegisterProcessor(proc, &SimpleMessage{})

	err = h1.Send(h2.address, &SimpleMessage{12})
	require.Nil(t, err)

	// Receive the message
	msg := <-proc.relay
	if msg.I != 12 {
		t.Fatal("Simple message got distorted")
	}

	h12 := h1.connection(h2.address)
	require.Len(t, h2.connections, 1)
	var h21 Conn
	for _, c := range h2.connections {
		h21 = c
	}

	assert.NotNil(t, h12)
	require.NotNil(t, h21)
	require.Nil(t, h21.Close())
	// let time to close the conn. on both sides
	time.Sleep(100 * time.Millisecond)
	err = h1.Send(h2.address, &SimpleMessage{12})
	require.Nil(t, err)
	<-proc.relay

	if err := h2.Stop(); err != nil {
		t.Fatal("Should be able to stop h2")
	}
	err = h1.Send(h2.address, &SimpleMessage{12})
	require.NotNil(t, err)

}

// Test connection of multiple Hosts and sending messages back and forth
// also tests for the counterIO interface that it works well
func TestRouterMessaging(t *testing.T) {
	h1, err1 := NewTestRouterTCP(2009)
	h2, err2 := NewTestRouterTCP(2010)
	if err1 != nil || err2 != nil {
		t.Fatal("Could not setup hosts")
	}

	go h1.Start()
	go h2.Start()

	defer func() {
		h1.Stop()
		h2.Stop()
		time.Sleep(250 * time.Millisecond)
	}()

	proc := &simpleMessageProc{t, make(chan SimpleMessage)}
	h1.RegisterProcessor(proc, SimpleMessage{})
	h2.RegisterProcessor(proc, SimpleMessage{})

	msgSimple := &SimpleMessage{3}
	err := h1.Send(h2.address, msgSimple)
	require.Nil(t, err)
	decoded := <-proc.relay
	assert.Equal(t, 3, decoded.I)

	// make sure the connection is registered in host1 (because it's launched in
	// a go routine). Since we try to avoid random timeout, let's send a msg
	// from host2 -> host1.
	assert.Nil(t, h2.Send(h1.address, msgSimple))
	decoded = <-proc.relay
	assert.Equal(t, 3, decoded.I)

	written := h1.Tx()
	read := h2.Rx()
	if written == 0 || read == 0 || written != read {
		log.Errorf("Tx = %d, Rx = %d", written, read)
		log.Errorf("h1.Tx() %d vs h2.Rx() %d", h1.Tx(), h2.Rx())
		log.Errorf("Something is wrong with Host.CounterIO")
	}
}

func TestRouterLotsOfConnTCP(t *testing.T) {
	testRouterLotsOfConn(t, NewTestRouterTCP, 5)
}

func TestRouterLotsOfConnLocal(t *testing.T) {
	testRouterLotsOfConn(t, NewTestRouterLocal, 2)
}

// nSquareProc will send back all packet sent and stop when it has received
// enough, it releases the waitgroup.
type nSquareProc struct {
	t           *testing.T
	r           *Router
	expected    int
	wg          *sync.WaitGroup
	firstRound  map[Address]bool
	secondRound map[Address]bool
	sync.Mutex
}

func newNSquareProc(t *testing.T, r *Router, expect int, wg *sync.WaitGroup) *nSquareProc {
	return &nSquareProc{t, r, expect, wg, make(map[Address]bool), make(map[Address]bool), sync.Mutex{}}
}

func (p *nSquareProc) Process(remote Address, msg Message) {
	p.Lock()
	defer p.Unlock()
	ok := p.firstRound[remote]
	if ok {
		// second round
		if ok := p.secondRound[remote]; ok {
			p.t.Fatal("Already received second round")
		}
		p.secondRound[remote] = true

		if len(p.secondRound) == p.expected {
			// release
			p.wg.Done()
		}
		return
	}

	p.firstRound[remote] = true
	if err := p.r.Send(remote, &SimpleMessage{3}); err != nil {
		p.t.Fatal("Could not send to first round dest.")
	}

}

// Makes a big mesh where every host send and receive to every other hosts
func testRouterLotsOfConn(t *testing.T, fac routerFactory, nbrRouter int) {
	// create all the routers
	routers := make([]*Router, nbrRouter)
	// to wait for the creation of all hosts
	var wg1 sync.WaitGroup
	wg1.Add(nbrRouter)
	var wg2 sync.WaitGroup
	wg2.Add(nbrRouter)
	for i := 0; i < nbrRouter; i++ {
		go func(j int) {
			r, err := fac(2000 + j)
			if err != nil {
				t.Fatal(err)
			}
			go r.Start()
			for !r.Listening() {
				time.Sleep(20 * time.Millisecond)
			}
			routers[j] = r
			// expect nbrRouter - 1 messages
			proc := newNSquareProc(t, r, nbrRouter-1, &wg2)
			r.RegisterProcessor(proc, SimpleMessage{})
			wg1.Done()
		}(i)
	}
	wg1.Wait()

	for i := 0; i < nbrRouter; i++ {
		j := i
		r := routers[j]
		for k := 0; k < nbrRouter; k++ {
			if k == j {
				// don't send to yourself
				continue
			}
			// send to everyone else
			if err := r.Send(routers[k].address, &SimpleMessage{3}); err != nil {
				t.Fatal(err)
			}
		}
	}
	wg2.Wait()
	for i := 0; i < nbrRouter; i++ {
		r := routers[i]
		log.Print("Closing ", r.address)
		if err := r.Stop(); err != nil {
			t.Fatal(err)
		}

	}
}

// Test sending data back and forth using the sendProtocolMsg
func TestRouterSendMsgDuplexTCP(t *testing.T) {
	testRouterSendMsgDuplex(t, NewTestRouterTCP)
}

func TestRouterSendMsgDuplexLocal(t *testing.T) {
	testRouterSendMsgDuplex(t, NewTestRouterLocal)
}
func testRouterSendMsgDuplex(t *testing.T, fac routerFactory) {
	h1, err1 := fac(2011)
	h2, err2 := fac(2012)
	if err1 != nil || err2 != nil {
		t.Fatal("Could not setup hosts")
	}
	go h1.Start()
	go h2.Start()

	defer func() {
		h1.Stop()
		h2.Stop()
		time.Sleep(250 * time.Millisecond)
	}()

	proc := &simpleMessageProc{t, make(chan SimpleMessage)}
	h1.RegisterProcessor(proc, SimpleMessage{})
	h2.RegisterProcessor(proc, SimpleMessage{})

	msgSimple := &SimpleMessage{5}
	err := h1.Send(h2.address, msgSimple)
	if err != nil {
		t.Fatal("Couldn't send message from h1 to h2", err)
	}
	msg := <-proc.relay
	log.Lvl2("Received msg h1 -> h2", msg)

	err = h2.Send(h1.address, msgSimple)
	if err != nil {
		t.Fatal("Couldn't send message from h2 to h1", err)
	}
	msg = <-proc.relay
	log.Lvl2("Received msg h2 -> h1", msg)
}

func TestRouterRxTx(t *testing.T) {
	RegisterMessage(60, basicMessage{})
	router1, err := NewTestRouterTCP(0)
	log.ErrFatal(err)
	router2, err := NewTestRouterTCP(0)
	log.ErrFatal(err)
	go router1.Start()
	go router2.Start()
	log.ErrFatal(router2.Send(router1.address, &basicMessage{10}))

	// Wait for the message to be sent and received
	waitTimeout(time.Second, 10, func() bool {
		return router1.Rx() > 0 && router1.Rx() == router2.Tx()
	})
	rx := router1.Rx()
	assert.Equal(t, rx, router1.Rx())
	assert.Equal(t, 1, len(router1.connections))
	router2.Stop()
	router1.Stop()
}

type EventReceiver struct {
	ch chan event.Event
}

func (e *EventReceiver) Receive(ev event.Event) {
	e.ch <- ev
}

func TestRouterPublish(t *testing.T) {
	t.Skip()
	log.TestOutput(true, 2)
	r1, err := NewTestRouterLocal(2000)
	require.Nil(t, err)
	go r1.Start()
	defer r1.Stop()

	r2, err := NewTestRouterLocal(2001)
	require.Nil(t, err)
	go r2.Start()

	rec := &EventReceiver{make(chan event.Event)}
	r1.Register(EventConnUp, rec)

	require.Nil(t, r2.Send(r1.address, &SimpleMessage{}))

	addr := r2.connections[r1.address].Local()
	select {
	case e := <-rec.ch:
		require.Equal(t, EventConnUp, e.Name())
		up, ok := e.(*EventUp)
		require.True(t, ok)
		require.Equal(t, addr, up.Address)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("too long")
	}

	r1.Register(EventConnDown, rec)
	require.Nil(t, r2.Stop())
	require.Error(t, r1.Send(r2.address, &SimpleMessage{}))
	select {
	case e := <-rec.ch:
		require.Equal(t, EventConnDown, e.Name())
		down, ok := e.(*EventDown)
		require.True(t, ok)
		require.Equal(t, addr, down.Address)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("too long")
	}

}

func waitTimeout(timeout time.Duration, repeat int,
	f func() bool) {
	success := make(chan bool)
	go func() {
		for !f() {
			time.Sleep(timeout / time.Duration(repeat))
		}
		success <- true
	}()
	select {
	case <-success:
	case <-time.After(timeout):
		log.Fatal("Timeout" + log.Stack())
	}

}
