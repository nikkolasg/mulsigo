package client

import (
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/dedis/onet/log"
	"github.com/nikkolasg/mulsigo/slog"
	"github.com/stretchr/testify/require"
)

func TestStreamNoiseHandshake(t *testing.T) {

	// create identities
	p1, i1, err := NewPrivateIdentity(name1, rand.Reader)
	require.Nil(t, err)
	p2, i2, err := NewPrivateIdentity(name2, rand.Reader)
	require.Nil(t, err)

	id, _ := channelID(i1, i2)
	r, channels := Channels(id, 2)
	defer r.Stop()
	c1, c2 := channels[0], channels[1]
	//c1, c2 := createFakeChannelPair()

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

func TestStreamNoiseDispatching(t *testing.T) {
	log.TestOutput(true, 5)
	// create identities
	p1, i1, err := NewPrivateIdentity(name1, rand.Reader)
	require.Nil(t, err)
	p2, i2, err := NewPrivateIdentity(name2, rand.Reader)
	require.Nil(t, err)

	// create a channel between the two identities
	id, _ := channelID(i1, i2)
	r, channels := Channels(id, 2)
	fmt.Println(r)
	defer func() { fmt.Println("Closing relay..."); r.Stop() }()
	c1, c2 := channels[0], channels[1]

	//	c1, c2 := createFakeChannelPair()

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
	//defer func() { fmt.Println("closing n1..."); n1.close() }()
	//defer func() { fmt.Println("closing n2..."); n2.close() }()

	<-handshakeDone
	<-handshakeDone
	//go dispatchStream(n1, i2, d)
	go dispatchStream(n2, i1, d)
	// sends a message n1 -> n2
	go func() {
		buf, _ := ClientEncoder.Marshal(&ClientMessage{})
		fmt.Println("n1.send() ---> ?")
		err := n1.send(buf)
		fmt.Println("n1.send() --->", err)
		require.Nil(t, err)
	}()
	select {
	case c := <-incoming:
		require.NotNil(t, c)
		fmt.Println("fine so far?")
	case <-time.After(1000 * time.Millisecond):
		t.FailNow()
	}
}

func TestStreamReliable(t *testing.T) {
	log.TestOutput(true, 5)
	//f1, f2 := NewFakeStreamsPair()
	// create identities
	p1, i1, err := NewPrivateIdentity(name1, rand.Reader)
	require.Nil(t, err)
	p2, i2, err := NewPrivateIdentity(name2, rand.Reader)
	require.Nil(t, err)

	id, _ := channelID(i1, i2)
	r, channels := Channels(id, 2)
	c1, c2 := channels[0], channels[1]

	n1 := newNoiseStream(p1, i1, i2, c1)
	n2 := newNoiseStream(p2, i2, i1, c2)
	go n1.doHandshake()
	go n2.doHandshake()
	r1 := newReliableStream(n1).(*reliableStream)
	//r2 := newReliableStream(n2).(*reliableStream)

	msg := []byte("Hello World")

	// test repeating ack
	defer func(defValue int) { MaxIncorrectMessages = defValue }(MaxIncorrectMessages)
	MaxIncorrectMessages = 2
	defer func(defValue time.Duration) { ReliableWaitRetry = defValue }(ReliableWaitRetry)
	ReliableWaitRetry = time.Millisecond * 100

	require.Error(t, r1.send(msg))
	require.Equal(t, uint32(1), r1.sequence)
	r.Stop()

}

func TestStreamReliableWorking(t *testing.T) {
	//log.TestOutput(true, 5)
	slog.Level = slog.LevelDebug
	msg2 := []byte("Hello Universe")
	p1, i1, err := NewPrivateIdentity(name1, rand.Reader)
	require.Nil(t, err)
	p2, i2, err := NewPrivateIdentity(name2, rand.Reader)
	require.Nil(t, err)

	//	f1, f2 = NewFakeStreamsPair()
	id, first := channelID(i1, i2)
	slog.Debug("Is identity 1 first ? ", first)
	r, channels := Channels(id, 2)
	defer r.Stop()
	c1, c2 := channels[0], channels[1]

	n1 := newNoiseStream(p1, i1, i2, c1)
	n2 := newNoiseStream(p2, i2, i1, c2)
	if first {
		go n2.doHandshake()
		require.Nil(t, n1.doHandshake())
	} else {
		go n1.doHandshake()
		require.Nil(t, n2.doHandshake())
	}

	r1 := newReliableStream(n1).(*reliableStream)
	r2 := newReliableStream(n2).(*reliableStream)
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
	slog.Debug("END OF TEST")
}

func dispatchStream(s stream, id *Identity, d Dispatcher) {
	for {
		buf, err := s.receive()
		fmt.Println("receiving fine ? ", err)
		if err != nil {
			return
		}
		cm, err := ClientEncoder.Unmarshal(buf)
		if err != nil {
			panic(err)
		}
		d.Dispatch(id, cm.(*ClientMessage))
	}
}
