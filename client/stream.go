package client

import (
	"errors"
	"sync"
	"time"

	"github.com/flynn/noise"
	"github.com/nikkolasg/mulsigo/network"
	"github.com/nikkolasg/mulsigo/relay"
	"github.com/nikkolasg/mulsigo/slog"
)

type stream interface {
	send(buff []byte) error
	receive() ([]byte, error)
	close()
}

var ErrTimeout = errors.New("timeout on stream")

// ReliableReadTimeout indicates how much time to wait for a "receive" operation
// using the reliable stream mechanism
var ReliableReadTimeout = 1 * time.Minute

// ReliableWriteTimeout indicates how much to wait for a "send" operation using
// the reliable stream meachanism
var ReliableWriteTimeout = 10 * time.Second

// ReliableMessageBuffer indicates the maximum number of messages the reliable
// stream meachanism can hold in memory for the application layer to read
var ReliableMessageBuffer = 100

// how many incorrect messages do we allow before returning an error. This is
// necessary since anybody could join and spam any channel.
var MaxIncorrectMessages = 100

// encoder of the "application" message layer
var enc = network.NewSingleProtoEncoder(ClientMessage{})

// encoder of the reliable stream packets
var reliableEncoder = network.NewSingleProtoEncoder(ReliablePacket{})

// cipherSuite used by the noise framework
var cipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2b)

//var cipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherAESGCM, noise.HashSHA256)

// magic value being the common string given in the noise handshake
var magicValue = []byte{0x19, 0x84}

type reliableStream struct {
	stream
	sequence  uint32
	ackCh     chan uint32
	dataCh    chan []byte
	errCh     chan bool // to be closed upon any error
	done      chan bool
	globalErr error
	errMut    sync.Mutex
	sendMut   sync.Mutex // only one send operation at a time
}

func newReliableStream(s stream) stream {
	return &reliableStream{
		stream: s,
		dataCh: make(chan []byte, ReliableMessageBuffer),
		errCh:  make(chan bool),
		ackCh:  make(chan uint32, ReliableMessageBuffer),
		done:   make(chan bool),
	}
}

func (r *reliableStream) stateMachine() {
	for {
		rec, err := r.stream.receive()
		if err != nil {
			r.setError(err)
			return
		}
		msg, err := reliableEncoder.Unmarshal(rec)
		if err != nil {
			r.setError(err)
			return
		}
		rp := msg.(*ReliablePacket)
		switch rp.Type {
		case RELIABLE_DATA:
			packet := &ReliablePacket{
				Type:     RELIABLE_ACK,
				Sequence: rp.Sequence,
			}
			encoded, err := reliableEncoder.Marshal(packet)
			if err != nil {
				panic("something's wrong with the encoding")
			}
			if err := r.stream.send(encoded); err != nil {
				r.setError(err)
				return
			}
			r.dataCh <- rp.Data
		case RELIABLE_ACK:
			r.ackCh <- rp.Sequence
		}
	}
}

func (r *reliableStream) send(buf []byte) error {
	r.sendMut.Lock()
	defer r.sendMut.Unlock()
	// get new sequence and send packet
	r.sequence++
	packet := &ReliablePacket{
		Type:     RELIABLE_DATA,
		Sequence: r.sequence,
		Data:     buf,
	}
	encoded, err := reliableEncoder.Marshal(packet)
	if err != nil {
		panic("something's wrong with the encoding")
	}
	if err := r.stream.send(encoded); err != nil {
		r.setError(err)
		return err
	}
	// try many times to get an ack
	for i := 0; i < MaxIncorrectMessages; i++ {
		if i != 0 {
			time.Sleep(1 * time.Second)
		}
		select {
		case ack := <-r.ackCh:
			if ack != packet.Sequence {
				continue
			}
			// all fine
			return nil
		case <-r.errCh:
			return r.getError()
		case <-time.After(10 * time.Second):
			i += MaxIncorrectMessages / 10
			continue
		}
	}
	return errors.New("reliable stream: no ack received in long time")
}

func (r *reliableStream) receive() ([]byte, error) {
	select {
	case data := <-r.dataCh:
		return data, nil
	case <-r.errCh:
		// could directly return the error but "write" also needs it anyway
		return nil, r.getError()
	case <-time.After(ReliableReadTimeout):
		return nil, ErrTimeout
	}
}

func (r *reliableStream) setError(e error) {
	r.errMut.Lock()
	defer r.errMut.Unlock()
	r.globalErr = e
	close(r.errCh)
}

func (r *reliableStream) getError() error {
	r.errMut.Lock()
	defer r.errMut.Unlock()
	return r.globalErr
}

func (r *reliableStream) close() {
	close(r.done)
}

const (
	// Noise_KK(s, rs):
	// -> s  msg1
	// <- s  msg2
	// -> e, es, ss  msg3
	// <- e, ee, se  done
	none = iota
	done
)

// noiseStream reads and sends message to/from a relay.Channel. Each message is
// encrypted using the Noise framework.
type noiseStream struct {
	// identity of the remote participant, needed for Noise communication
	remote *Identity
	// first denotes who is supposed to send the first message
	first bool
	// noise related variables
	handshake     *noise.HandshakeState
	encrypt       *noise.CipherState
	decrypt       *noise.CipherState
	handshakeDone bool
	// the underlying channel where to send data to
	channel relay.Channel
	// the dispatcher where to relay incoming messages from the channel
	dispatcher Dispatcher
}

// newNoiseStream returns a stream encrypting messages using the noise
// framework. After that call, the caller must call "stream.doHandshake()` and
// verify the return error.
func newNoiseStream(priv *Private, pub, remote *Identity, ch relay.Channel, d Dispatcher) *noiseStream {
	str := &noiseStream{
		remote:     remote,
		channel:    ch,
		dispatcher: d,
	}
	_, str.first = channelID(pub, remote)

	// convert ed25519 private / public keys to curve25519
	privCurve := priv.PrivateCurve25519()
	pubCurve := pub.PublicCurve25519()
	remotePublic := remote.PublicCurve25519()
	kp1 := noise.DHKey{
		Private: privCurve[:],
		Public:  pubCurve[:],
	}

	str.handshake = noise.NewHandshakeState(noise.Config{
		CipherSuite:   cipherSuite,
		Pattern:       noise.HandshakeKK,
		Initiator:     str.first,
		StaticKeypair: kp1,
		PeerStatic:    remotePublic[:],
	})

	return str
}

// doHandshake must be called before any Send or listen method call. It
// dispatches to dohandshakeClient or doHandshakeServer according to which
// party's must send the first handshake message first.
func (n *noiseStream) doHandshake() error {
	if n.handshakeDone {
		return errors.New("noise handshake already in process")
	}

	// terminology of TLS implementations:
	// client is sending the first message
	var err error
	if n.first {
		err = n.doHandshakeClient()
	} else {
		err = n.doHandshakeServer()
	}
	if err == nil {
		n.handshakeDone = true
	}
	return err
}

func (n *noiseStream) doHandshakeClient() error {
	msg, _, _ := n.handshake.WriteMessage(nil, nil)
	if err := n.channel.Send(msg); err != nil {
		return err
	}
	enc, dec, err := n.receiveHandshake()
	if err != nil {
		return err
	}

	n.encrypt = enc
	n.decrypt = dec
	return nil
}

func (n *noiseStream) doHandshakeServer() error {
	_, _, err := n.receiveHandshake()
	if err != nil {
		return err
	}

	res, dec, enc := n.handshake.WriteMessage(nil, magicValue)
	if err != nil {
		return err
	}

	if err := n.channel.Send(res); err != nil {
		return err
	}
	n.encrypt = enc
	n.decrypt = dec
	return nil
}

// receiveHandshake tries to receive a correct handshake message a
// "maxIncorrectMessage" number of times.
func (n *noiseStream) receiveHandshake() (*noise.CipherState, *noise.CipherState, error) {
	for i := 0; i < MaxIncorrectMessages; i++ {
		if i != 0 {
			time.Sleep(1 * time.Second)
		}
		_, buff, err := n.channel.Receive()
		if err != nil {
			continue
		}
		_, enc, dec, err := n.handshake.ReadMessage(nil, buff)
		if err != nil {
			continue
		}
		return enc, dec, err
	}
	return nil, nil, errors.New("noise: can't validate one out of 100 handshake. Probably being DOS'd")
}

// Sends takes a message encrypts it using the noise framework and sends it down
// to the channel.
func (n *noiseStream) send(msg []byte) error {
	if !n.handshakeDone {
		return errors.New("noiseStream: doHandshake() not called before Send()")
	}
	cipher := n.encrypt.Encrypt(nil, nil, msg)
	return n.channel.Send(cipher)
}

// listen is an infinite loop listening for incoming messages from the
// underlying channel, decrypting them using Noise and dispatching them to the
// Dispatcher. It stops when `stop()` is called.
func (n *noiseStream) receive() ([]byte, error) {
	_, buff, err := n.channel.Receive()
	if err != nil {
		slog.Print("noise: interruption", err)
		return nil, err
	}
	return n.decrypt.Decrypt(nil, nil, buff)
}

func (n *noiseStream) close() {
	n.channel.Close()
}
