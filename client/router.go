package client

import (
	"crypto/sha256"
	"strings"
	"sync"

	"github.com/flynn/noise"
	"github.com/nikkolasg/mulsigo/network"
	"github.com/nikkolasg/mulsigo/relay"
	"github.com/nikkolasg/mulsigo/slog"
	"github.com/nikkolasg/mulsigo/util"
)

// Router is an interface whose main purpose is to provide a generic mean to
// receive and send message to a Client whether a relay or not is used. This
// will be useful to transform the "every message goes to relay"-approach to a
// "point-to-point communication"-approach.
type Router interface {
	Send(Identity, *ClientMessage) error
	Receive() (Identity, *ClientMessage, error)
	Broadcast(cm *ClientMessage, ids ...Identity) error
	// push a message to the Router to be popped off with Receive()
	push(string, *ClientMessage)
}

var cipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2b)

// BiLinkRouter establish a relay channel for each destination address
// It implements the client.Router interface.
type clientRouter struct {
	// the identity advertised by this router
	id Identity
	// multiplexer used to derive the channels
	multiplexer *relay.Multiplexer
	// list of open channels maintained by the router
	channels    map[string]*channel
	channelsMut sync.Mutex

	// incoming messages waiting to be read by Receive()
	incoming chan message

	doneMut sync.Mutex
	done    bool
}

// NewRelayRouter returns a router that communicates with a relay in a
// transparent way to the given address.
func NewClientRouter(own Identity, m *relay.Multiplexer) Router {
	blr := &clientRouter{
		id:          own,
		multiplexer: m,
		channels:    make(map[string]*relay.Channel),
		incoming:    make(chan message, 50),
	}
	return blr
}

func (blr *clientRouter) Send(remote *Identity, msg interface{}) error {
	blr.Lock()
	defer blr.Unlock()
	id, first, err := channelID(blr.id, remote)
	if err != nil {
		return err
	}

	channel, ok := blr.channels[id]
	if !ok {
		channel, err = blr.multiplexer.Channel(id)
		if err != nil {
			return err
		}
		noiseWr := newNoiseWrapper(channel, remote, first)
		blr.channels[id] = newStream(blr, channel, noiseWr)
	}
	channel.Send(msg)
}

const (
	// Noise_KK(s, rs):
	// -> s  msg1
	// <- s  msg2
	// -> e, es, ss  msg3
	// <- e, ee, se  done
	none = iota
	msg1
	msg2
	msg3
	done
)

//         bytes             bytes           encoding/ interfaces
// network <-> relay.Channel <-> noiseWrapper <->  stream
// wrapper is the intermediate between the higher level interface{} stream and
// the low level bytes relay.Channel. It can be used to add an encryption level,
// padding etc...
type wrapper interface {
	wrap([]byte) error
	unwrap() (string, []byte, error)
}

// noiseWrapper applies the Noise framework to the stream.
type noiseWrapper struct {
	remote        *Identity
	handshake     *noise.HandshakeState
	handshakeStep int
	channel       relay.Channel
}

func newNoiseWrapper(ch relay.Channel, own, remote *Identity, init bool) *noiseWrapper {
	pub, _ := own.Key.MarshalBinary()
	hs := noise.NewHandshakeState(noise.Config{
		CipherSuite:   cipherSuite,
		Pattern:       noise.HandshakeKK,
		Initiator:     init,
		StaticKeypair: pub,
	})

	return noiseWrapper{
		remote:    remote,
		handshake: hs,
		channel:   ch,
	}
}

func (nw *noiseWrapper) wrap(buff []byte) error {
	if nw.handshakeStep != done {
		return nw.doHandshake()
	}
}

func (nw *noiseWrapper) unwrap() (string, []byte, error) {

}

func (nw *noiseWrapper) doHandshake() {
	switch nw.handshakeStep {
	case none:
		hello, _, _ := nw.handshake.WriteMessage(nil, nil)
		return nw.channel.Send(hello)
	}
}

type naclWrapper struct {
}

var enc = network.NewSingleProtoEncoder(ClientMessage{})

type stream struct {
	channelID string
	remote    *Identity
	w         wrapper
	r         Router
}

func newStream(r Router, ch relay.Channel, wr wrapper) *stream {
	str := &stream{
		channelID: id.Id(),
		remote:    remote,
		r:         r,
		w:         wr,
	}
	go str.listen()
	return str
}

func (str *stream) send(remote *Identity, msg *ClientMessage) error {
	buff, err := enc.Marshal(msg)
	if err != nil {
		return err
	}
	return str.w.wrap(buff)
}

func (str *stream) listen() {
	for !str.stopped() {
		from, buff, err := str.w.unwrap()
		if err != nil {
			slog.Print("stream interrupted:", err)
			return
		}
		env, err := enc.Unmarshal(buff)
		if err != nil {
			continue
		}
		cm, ok := env.(*ClientMessage)
		if !ok {
			continue
		}
		str.r.push(from, cm)
	}
}

func (str *stream) stopped() bool {
	str.stopMut.Lock()
	defer str.stopMut.Unlock()
	return str.stopped
}

func (str *stream) stop() {
	str.stopMut.Lock()
	defer str.stopMut.Unlock()
	str.stopped = true
}

// channelID returns the channel id associated with the two given identity. It's
// basically base64-encoded and sorted, then hashed. The second return value
// denotes if own is first or not (useful to designate initiator).
func channelID(own, remote Identity) (string, bool) {
	str1, _ := util.PointToString64(own.Public)
	str2, _ := util.PointToString64(remote.Public)
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
