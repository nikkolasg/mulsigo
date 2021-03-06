package client

import (
	"errors"
	"math"
	"sync"
	"time"

	"github.com/flynn/noise"
	"github.com/nikkolasg/mulsigo/network"
	"github.com/nikkolasg/mulsigo/relay"
	"github.com/nikkolasg/mulsigo/slog"
)

// stream is an abstraction to represent any ways to connect to a remote peer.
// The equivalent of a net.Con interface.
// When using a channel+noiseStream as the underlying communication medium, a
// stream is created when both parties have connected to the same channel id and
// have finished the acknowledgment phase.
// When using a direct connection, each party knows if it must listen or
// connect, then the noise handshake happens and finally the stream is returned.
type stream interface {
	send(buff []byte) error
	receive() ([]byte, error)
	close()
}

// streamFactory abstracts the creation of streams so the Router can use streams
// without any knowledge of the underlying implementation.
type streamFactory interface {
	newStream(to *Identity) (stream, error)
}

// ErrTimeout is returned when a timeout has occured on a stream
var ErrTimeout = errors.New("stream: timeout occured")

// ErrClosed is returned when one call send() or receive() over a stream while
// the stream is closed
var ErrClosed = errors.New("stream: closed")

// ReliableReadTimeout indicates how much time to wait for a "receive" operation
// using the reliable stream mechanism
var ReliableReadTimeout = 1 * time.Minute

// ReliableWriteTimeout indicates how much to wait for a "send" operation using
// the reliable stream meachanism
var ReliableWriteTimeout = 10 * time.Second

// ReliableMessageBuffer indicates the maximum number of messages the reliable
// stream meachanism can hold in memory for the application layer to read
var ReliableMessageBuffer = 100

// ReliableWaitRetry indicates the idle period between re-sending a packet to
// get an ACK
var ReliableWaitRetry = 1 * time.Second

// how many incorrect messages do we allow before returning an error. This is
// necessary since anybody could join and spam any channel.
var MaxIncorrectMessages = 100

// encoder of the "application" message layer
var ClientEncoder = network.NewSingleProtoEncoder(ClientMessage{})

// encoder of the "relay" messages for relay streams
var relayEncoder = network.NewSingleProtoEncoder(relay.RelayMessage{})

// encoder of the reliable stream packets
var reliableEncoder = network.NewSingleProtoEncoder(ReliablePacket{})

// cipherSuite used by the noise framework
var cipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2b)

//var cipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherAESGCM, noise.HashSHA256)

// magic value being the common string given in the noise handshake
var magicValue = []byte{0x19, 0x84}

// channelStreamFactory creates reliable stream based on relay.Channel
type channelStreamFactory struct {
	priv        *Private
	pub         *Identity
	multiplexer *relay.Multiplexer
}

// newChannelStreamFactory returns a streamFactory that returns channel-based
// stream with additional reliability handled by reliableStream.
func newChannelStreamFactory(priv *Private, mult *relay.Multiplexer) *channelStreamFactory {
	return &channelStreamFactory{
		priv:        priv,
		pub:         priv.Public,
		multiplexer: mult}
}

func (c *channelStreamFactory) newStream(to *Identity) (stream, error) {
	// XXX would be nice to remove some of the repetitive computations ...
	id, _ := channelID(c.pub, to)
	// create the channel abstraction
	channel, err := c.multiplexer.Channel(id)
	if err != nil {
		return nil, err
	}

	stream := newNoiseStream(c.priv, c.pub, to, channel)
	slog.Debug("streamFactory: ", c.pub.ID(), " TRYING handshake with ", to.ID())
	if err := stream.doHandshake(); err != nil {
		return nil, err
	}
	slog.Debug("streamFactory: ", c.pub.ID(), " handshake done with ", to.ID())
	reliable := newReliableStream(stream)
	go reliable.stateMachine()
	return reliable, nil
}

type connStreamFactory struct {
	// XXX TODO
}

// reliableStream is a stream built on top of another stream that provides super
// basic properties like ordering and reliability over the given stream.
// It sends and expects protobuf-encoded ReliableMessage.
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

func newReliableStream(s stream) *reliableStream {
	return &reliableStream{
		stream: s,
		dataCh: make(chan []byte, ReliableMessageBuffer),
		errCh:  make(chan bool),
		ackCh:  make(chan uint32, ReliableMessageBuffer),
		done:   make(chan bool),
	}
}

func (r *reliableStream) stateMachine() {
	var lastAckd uint32
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
				panic("something's wrong with the encoding" + err.Error())
			}
			slog.Debugf("reliable: got data with seq %d", rp.Sequence)
			if err := r.stream.send(encoded); err != nil {
				r.setError(err)
				return
			}
			slog.Debugf("reliable: sent back ACK %d", rp.Sequence)

			// pass unique message to the layer above
			// linearly increasing sequence number

			if rp.Sequence <= lastAckd {
				continue
			}
			lastAckd = rp.Sequence
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
		panic("something's wrong with the encoding" + err.Error())
	}
	slog.Debugf("reliable: sending packet with sequence %d", r.sequence)
	if err := r.stream.send(encoded); err != nil {
		r.setError(err)
		return err
	}
	// try many times to get an ack
	for i := 0; i < MaxIncorrectMessages; i++ {
		slog.Debugf("reliable: %d waiting ack %d", i, r.sequence)
		select {
		case ack := <-r.ackCh:
			// only handling one packet at a time for the moment, no window
			// mechanism
			if ack != packet.Sequence {
				continue
			}
			// all fine
			return nil
		case <-r.errCh:
			return r.getError()
		case <-r.done:
			return ErrClosed
		case <-time.After(ReliableWaitRetry):
			i += int(math.Ceil(float64(MaxIncorrectMessages) / 10.0))
			// re-send it again
			if err := r.stream.send(encoded); err != nil {
				r.setError(err)
				return err
			}
		}

	}
	return ErrTimeout
}

func (r *reliableStream) receive() ([]byte, error) {
	select {
	case data := <-r.dataCh:
		return data, nil
	case <-r.errCh:
		// could directly return the error but "write" also needs it anyway
		return nil, r.getError()
	case <-r.done:
		return nil, ErrClosed
	case <-time.After(ReliableReadTimeout):
		return nil, ErrTimeout
	}
}

func (r *reliableStream) setError(e error) {
	r.errMut.Lock()
	defer r.errMut.Unlock()
	if r.globalErr != nil {
		return
	}
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
	r.stream.close()
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
// this stream sends and expects protobuf-encoded relay.RelayMessage messages.
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
func newNoiseStream(priv *Private, pub, remote *Identity, ch relay.Channel) *noiseStream {
	str := &noiseStream{
		remote:  remote,
		channel: ch,
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
		return errors.New("noise: handshake already done or in process")
	}

	// terminology of TLS implementations:
	// client is sending the first message
	var err error
	if n.first {
		slog.Debugf("noise: starting handshake client")
		err = n.doHandshakeClient()
	} else {
		slog.Debugf("noise: starting handshake server")
		err = n.doHandshakeServer()
	}
	if err == nil {
		n.handshakeDone = true
		slog.Debugf("noise: handshake done !")
	}
	return err
}

func (n *noiseStream) doHandshakeClient() error {
	slog.Debugf("noise: handshake client send init message")
	msg, _, _ := n.handshake.WriteMessage(nil, nil)
	if err := n.channel.Send(msg); err != nil {
		return err
	}
	enc, dec, err := n.receiveHandshake()
	if err != nil {
		return err
	}

	slog.Debugf("noise: handshake client received handshake message")
	n.encrypt = enc
	n.decrypt = dec
	return nil
}

func (n *noiseStream) doHandshakeServer() error {
	slog.Debugf("noise: handshake server receiving init message...")
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
	slog.Debugf("noise: handhshake server sent back handshake message")
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
