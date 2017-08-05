package client

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
	require.Len(t, f2.buffers, 2)
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
	done := make(chan bool)
	go func() {
		require.Nil(t, r1.send(msg2))
		done <- true
	}()
	go func() {
		require.Nil(t, r2.send(msg2))
		done <- true
	}()

	_, err = r1.receive()
	require.Nil(t, err)
	_, err = r2.receive()
	require.Nil(t, err)
	require.Equal(t, uint32(2), r1.sequence)
	require.Equal(t, uint32(1), r2.sequence)

	<-done
	<-done

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
