package client

import (
	"crypto/sha256"
	"errors"
	"strings"
	"sync"

	"github.com/flynn/noise"
	"github.com/nikkolasg/mulsigo/network"
	"github.com/nikkolasg/mulsigo/relay"
)

// Router is an interface whose main purpose is to provide a generic mean to
// receive and send message to a Client whether a relay or not is used. This
// will be useful to transform the "every message goes to relay"-approach to a
// "point-to-point communication"-approach.
type Router interface {
	Send(Identity, *ClientMessage) error
	Receive() (Identity, *ClientMessage, error)
	Broadcast(cm *ClientMessage, ids ...Identity) error
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
	streams    map[string]*noiseStream
	streamsMut sync.Mutex

	doneMut sync.Mutex
	done    bool
}

// NewRelayRouter returns a router that communicates with a relay in a
// transparent way to the given address.
/*func NewClientRouter(priv *Private, pub *Identity, m *relay.Multiplexer) Router {*/
//blr := &clientRouter{
//priv:        priv,
//id:          pub,
//multiplexer: m,
//streams:     make(map[string]*noiseStream),
//}
//return blr
//}

//func (blr *clientRouter) Send(remote *Identity, msg interface{}) error {
//blr.Lock()
//defer blr.Unlock()
//id, first, err := channelID(blr.id, remote)
//if err != nil {
//return err
//}

//stream, ok := blr.streams[remote.ID()]
//if !ok {
//channel, err = blr.multiplexer.Channel(id)
//if err != nil {
//return err
//}
//stream = newStream(blr.priv, blr.id, remote, channel)
//blr.streams[id] = stream
//}
//channel.Send(msg)
//}

//func (blr *clientRouter) Broadcast(m

const (
	// Noise_KK(s, rs):
	// -> s  msg1
	// <- s  msg2
	// -> e, es, ss  msg3
	// <- e, ee, se  done
	none = iota
	hello
	done
)

var enc = network.NewSingleProtoEncoder(ClientMessage{})

//var cipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2b)
var cipherSuite = noise.NewCipherSuite(noise.DH25519, noise.CipherAESGCM, noise.HashSHA256)

type noiseStream struct {
	channelID     string
	remote        *Identity
	handshake     *noise.HandshakeState
	handshakeDone bool
	first         bool
	channel       relay.Channel
	encrypt       *noise.CipherState
	decrypt       *noise.CipherState
}

func newNoiseStream(priv *Private, pub, remote *Identity, ch relay.Channel) *noiseStream {
	str := &noiseStream{
		channelID: ch.Id(),
		remote:    remote,
		channel:   ch,
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

var magicValue = []byte{0x19, 0x84}

func (n *noiseStream) doHandshakeClient() error {
	msg, _, _ := n.handshake.WriteMessage(nil, nil)
	if err := n.channel.Send(msg); err != nil {
		return err
	}
	eg, err := n.channel.Receive()
	if err != nil {
		return err
	}
	_, enc, dec, err := n.handshake.ReadMessage(nil, eg.GetBlob())
	if err != nil {
		return err
	}

	n.encrypt = enc
	n.decrypt = dec
	return nil
}

func (n *noiseStream) doHandshakeServer() error {

	eg, err := n.channel.Receive()
	if err != nil {
		return err
	}

	_, _, _, err = n.handshake.ReadMessage(nil, eg.GetBlob())
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

/*func (str *stream) send(remote *Identity, msg *ClientMessage) error {*/
//buff, err := enc.Marshal(msg)
//if err != nil {
//return err
//}
//return str.w.wrap(buff)
//}

/*func (str *stream) listen() {*/
//for !str.stopped() {
//from, buff, err := str.w.unwrap()
//if err != nil {
//slog.Print("stream interrupted:", err)
//return
//}
//env, err := enc.Unmarshal(buff)
//if err != nil {
//continue
//}
//cm, ok := env.(*ClientMessage)
//if !ok {
//continue
//}
//str.r.push(from, cm)
//}
//}

//func (str *stream) stopped() bool {
//str.stopMut.Lock()
//defer str.stopMut.Unlock()
//return str.stopped
//}

//func (str *stream) stop() {
//str.stopMut.Lock()
//defer str.stopMut.Unlock()
//str.stopped = true
//}

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
