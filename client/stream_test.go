package client

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/nikkolasg/mulsigo/relay"
	"github.com/stretchr/testify/require"
)

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

	n1 := newNoiseStream(p1, i1, i2, c1, nil)
	n2 := newNoiseStream(p2, i2, i1, c2, nil)

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
	n1 := newNoiseStream(p1, i1, i2, c1, d)
	n2 := newNoiseStream(p2, i2, i1, c2, d)
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
