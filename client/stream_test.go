package client

import (
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/nikkolasg/mulsigo/relay"
	"github.com/stretchr/testify/require"
)

type fakeStream struct {
	buffers chan []byte
	remote  *fakeStream
}

func NewFakeStreamsPair() (*fakeStream, *fakeStream) {
	f1 := &fakeStream{buffers: make(chan []byte, 10)}
	f2 := &fakeStream{buffers: make(chan []byte, 10)}
	f1.remote = f2
	f2.remote = f1
	return f1, f2
}

func (f *fakeStream) send(buff []byte) error {
	f.remote.buffers <- buff
	return nil
}

func (f *fakeStream) receive() ([]byte, error) {
	b, ok := <-f.buffers
	if !ok {
		return nil, errors.New("fakestream closed")
	}
	return b, nil
}

func (f *fakeStream) close() {
	//
}

type fakeChannel struct {
	addr   string
	own    chan relay.Egress
	remote *fakeChannel
}

func createFakeChannelPair() (relay.Channel, relay.Channel) {
	c1 := &fakeChannel{addr: cAddr1.String(), own: make(chan relay.Egress, 50)}
	c2 := &fakeChannel{addr: cAddr2.String(), own: make(chan relay.Egress, 50)}
	c1.remote = c2
	c2.remote = c1
	return c1, c2
}

func (f *fakeChannel) Send(b []byte) error {
	f.remote.own <- relay.Egress{
		Address: f.addr,
		Blob:    b,
	}
	return nil
}

func (f *fakeChannel) Receive() (string, []byte, error) {
	e := <-f.own
	return e.GetAddress(), e.GetBlob(), nil
}

func (f *fakeChannel) Id() string {
	return "fake"
}

func (f *fakeChannel) Close() {
	close(f.own)
}

func TestNoiseStreamHandshake(t *testing.T) {

	// create identities
	p1, i1, err := NewPrivateIdentity(name1, rand.Reader)
	require.Nil(t, err)
	p2, i2, err := NewPrivateIdentity(name2, rand.Reader)
	require.Nil(t, err)

	// create a fake channel between the two identities
	c1, c2 := createFakeChannelPair()

	n1 := newNoiseStream(p1, i1, i2, c1)
	n2 := newNoiseStream(p2, i2, i1, c2)

	var ch = make(chan error)
	go func() {
		err := n1.doHandshake()
		ch <- err
	}()

	err = n2.doHandshake()
	require.Nil(t, err)
	require.Nil(t, <-ch)

	require.Error(t, n2.doHandshake())
}

func TestNoiseStreamDispatching(t *testing.T) {
	// create identities
	p1, i1, err := NewPrivateIdentity(name1, rand.Reader)
	require.Nil(t, err)
	p2, i2, err := NewPrivateIdentity(name2, rand.Reader)
	require.Nil(t, err)

	// create a fake channel between the two identities
	c1, c2 := createFakeChannelPair()

	// create receiving processor
	var incoming = make(chan *ClientMessage)
	var proc processor
	proc = func(i *Identity, c *ClientMessage) {
		incoming <- c
	}
	d := newSeqDispatcher()
	d.RegisterProcessor(&proc)

	// create the noise streams
	n1 := newNoiseStream(p1, i1, i2, c1)
	n2 := newNoiseStream(p2, i2, i1, c2)
	// do the handshake
	var handshakeDone = make(chan bool)
	go func() { err := n1.doHandshake(); require.Nil(t, err); handshakeDone <- true }()
	go func() { err := n2.doHandshake(); require.Nil(t, err); handshakeDone <- true }()
	defer n1.close()
	defer n2.close()
	<-handshakeDone
	<-handshakeDone
	go dispatchStream(n1, i2, d)
	go dispatchStream(n2, i1, d)
	// sends a message n1 -> n2
	go func() {
		buf, _ := enc.Marshal(&ClientMessage{})
		require.Nil(t, n1.send(buf))
	}()

	select {
	case c := <-incoming:
		require.NotNil(t, c)
	case <-time.After(1000 * time.Millisecond):
		t.FailNow()
	}
}

func TestReliableStream(t *testing.T) {
	f1, f2 := NewFakeStreamsPair()
	r1 := newReliableStream(f1).(*reliableStream)
	r2 := newReliableStream(f2).(*reliableStream)

	msg := []byte("Hello World")
	msg2 := []byte("Hello Universe")

	// test repeating ack
	defer func(defValue int) { MaxIncorrectMessages = defValue }(MaxIncorrectMessages)
	MaxIncorrectMessages = 2
	defer func(defValue time.Duration) { ReliableWaitRetry = defValue }(ReliableWaitRetry)
	ReliableWaitRetry = time.Millisecond * 100

	require.Error(t, r1.send(msg))
	require.Len(t, f2.buffers, 3)
	require.Equal(t, uint32(1), r1.sequence)

	f1, f2 = NewFakeStreamsPair()
	r1 = newReliableStream(f1).(*reliableStream)
	r2 = newReliableStream(f2).(*reliableStream)

	go r1.stateMachine()
	go r2.stateMachine()
	// test sending a message
	require.Nil(t, r1.send(msg2))
	buff, err := r2.receive()
	require.Nil(t, err)
	require.Equal(t, msg2, buff)
	require.Equal(t, uint32(1), r1.sequence)

	// test sending messages simultaneously
	go func() { require.Nil(t, r1.send(msg2)) }()
	go func() { require.Nil(t, r2.send(msg2)) }()

	_, err = r1.receive()
	require.Nil(t, err)
	_, err = r2.receive()
	require.Nil(t, err)
	require.Equal(t, uint32(2), r1.sequence)
	require.Equal(t, uint32(1), r2.sequence)

	// test closing then sending
	r1.close()
	require.Error(t, r1.send(msg2))
}

func dispatchStream(s stream, id *Identity, d Dispatcher) {
	for {
		buf, err := s.receive()
		if err != nil {
			return
		}
		cm, err := enc.Unmarshal(buf)
		if err != nil {
			panic(err)
		}
		d.Dispatch(id, cm.(*ClientMessage))
	}
}
