package client

import (
	"crypto/sha256"
	"errors"
	"strings"
	"sync"

	"github.com/flynn/noise"
	"github.com/nikkolasg/mulsigo/network"
	"github.com/nikkolasg/mulsigo/relay"
	"github.com/nikkolasg/mulsigo/slog"
)

// Router is an interface whose main purpose is to provide a generic mean to
// receive and send message to a Client whether a relay or not is used. This
// will be useful to transform the "every message goes to relay"-approach to a
// "point-to-point communication"-approach.
type Router interface {
	Dispatcher
	Send(*Identity, *ClientMessage) error
	Broadcast(cm *ClientMessage, ids ...*Identity) error
	Close()
}

// Dispatcher receives messages and dispatch them to the registered processors.
// If multiple processors are registered, each message is dispatched to each
// processors.
type Dispatcher interface {
	RegisterProcessor(Processor)
	Dispatch(*Identity, *ClientMessage)
}

// Processor can receive a message from a dispatcher.
type Processor interface {
	Process(*Identity, *ClientMessage)
}

// BiLinkRouter establish a relay channel for each destination address
// It implements the client.Router interface.
type clientRouter struct {
	// the private key material
	priv *Private
	// the identity advertised by this router
	id *Identity
	// multiplexer used to derive the channels
	multiplexer *relay.Multiplexer
	// list of open streams maintained by the router
	streams map[string]*noiseStream
	sync.Mutex

	// dispatcher used to dispatch message
	Dispatcher

	doneMut sync.Mutex
	done    bool
}

// NewRelayRouter returns a router that communicates with a relay in a
// transparent way to the given address.
func NewClientRouter(priv *Private, pub *Identity, m *relay.Multiplexer) Router {
	blr := &clientRouter{
		priv:        priv,
		id:          pub,
		multiplexer: m,
		streams:     make(map[string]*noiseStream),
		Dispatcher:  newSeqDispatcher(),
	}
	return blr
}

// Send fetch or create the corresponding channel corresponding to the pair tied
// to the given remote identity. It then sends the message down that channel.
func (blr *clientRouter) Send(remote *Identity, msg *ClientMessage) error {
	blr.Lock()
	defer blr.Unlock()
	id, _ := channelID(blr.id, remote)

	stream, ok := blr.streams[remote.ID()]
	if !ok {
		// create the channel abstraction
		channel, err := blr.multiplexer.Channel(id)
		if err != nil {
			return err
		}

		stream = newNoiseStream(blr.priv, blr.id, remote, channel, blr)
		blr.streams[id] = stream
		if err := stream.doHandshake(); err != nil {
			return err
		}
		go stream.listen()
	}
	return stream.Send(msg)
}

// Broadcast sends the message to all destinations given in ids. It returns as
// soon as an error is detected.
func (blr *clientRouter) Broadcast(msg *ClientMessage, ids ...*Identity) error {
	for _, i := range ids {
		if err := blr.Send(i, msg); err != nil {
			return err
		}
	}
	return nil
}

// Close will close all registered streams.
func (blr *clientRouter) Close() {
	blr.Lock()
	defer blr.Unlock()
	for _, s := range blr.streams {
		s.stop()
	}
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

var enc = network.NewSingleProtoEncoder(ClientMessage{})

var cipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2b)

//var cipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherAESGCM, noise.HashSHA256)

// magic value being the common string given in the noise handshake
var magicValue = []byte{0x19, 0x84}

// how many incorrect messages do we allow before returning an error. This is
// necessary since anybody could join the channel.
var maxIncorrectMessages = 100

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

	// is this noiseStream finished or not
	stopped bool
	stopMut sync.Mutex
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
	for i := 0; i < maxIncorrectMessages; i++ {
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
func (n *noiseStream) Send(msg *ClientMessage) error {
	if !n.handshakeDone {
		return errors.New("noiseStream: doHandshake() not called before Send()")
	}
	buff, err := enc.Marshal(msg)
	if err != nil {
		return err
	}
	cipher := n.encrypt.Encrypt(nil, nil, buff)
	return n.channel.Send(cipher)
}

// listen is an infinite loop listening for incoming messages from the
// underlying channel, decrypting them using Noise and dispatching them to the
// Dispatcher. It stops when `stop()` is called.
func (n *noiseStream) listen() {
	for !n.isStopped() {
		_, buff, err := n.channel.Receive()
		if err != nil {
			slog.Print("noise: interruption", err)
			return
		}
		plain, err := n.decrypt.Decrypt(nil, nil, buff)
		// incorrect messages are just no-op
		if err != nil {
			continue
		}

		env, err := enc.Unmarshal(plain)
		if err != nil {
			slog.Print("noise: received incorrect protobuf message")
			continue
		}
		cm, ok := env.(*ClientMessage)
		if !ok {
			slog.Print("noise: received incorrect protobuf message")
			continue
		}
		n.dispatcher.Dispatch(n.remote, cm)
	}
}

// isStopped returns whether this stream is closed or not.
func (str *noiseStream) isStopped() bool {
	str.stopMut.Lock()
	defer str.stopMut.Unlock()
	return str.stopped
}

// stop closes the stream and the underlying channel.
func (str *noiseStream) stop() {
	str.stopMut.Lock()
	defer str.stopMut.Unlock()
	str.channel.Close()
	str.stopped = true
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
