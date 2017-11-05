package relay

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	net "github.com/nikkolasg/mulsigo/network"
	"github.com/nikkolasg/mulsigo/slog"
)

// Channel is an abstraction representing a relay-allocated "room" where
// participants can broadcast messages. A Channel is identified by an id on the
// relay. To join a Channel, it is first required to send a JOIN_MESSAGE with
// the desired channel id. A Channel is a bidirectionel 1-to-many communication
// stream which offers NO GUARANTEES such as reliable delivery, etc.
type Channel interface {
	Send([]byte) error
	Receive() (string, []byte, error)
	Id() string
	Close()
}

// Multiplexer handles different communication "channel" on one underlying
// connection to a relay.
type Multiplexer struct {
	conn net.Conn

	channels map[string]*clientChannel
	chanMut  sync.Mutex

	stop chan bool
}

func NewMultiplexer(c net.Conn) *Multiplexer {
	m := &Multiplexer{
		conn:     c,
		stop:     make(chan bool),
		channels: make(map[string]*clientChannel),
	}
	go m.routine()
	return m
}

// Channel takes an id and returns the corresponding Channel. If the channel
// does not exists, it creates it and join the channel on the relay. It returns
// an error in case the "join" operation failed.
func (m *Multiplexer) Channel(id string) (Channel, error) {
	hexId := hex.EncodeToString([]byte(id))[:10]
	slog.Debugf("multiplexer: new channel %s joining...", hexId)
	m.chanMut.Lock()
	if s, ok := m.channels[id]; ok {
		m.chanMut.Unlock()
		return s, nil
	}
	ch := newClientChannel(id, m)
	m.channels[id] = ch
	m.chanMut.Unlock()

	err := ch.join()
	slog.Debugf("multiplexer: new channel %s joined!", hexId)
	if err != nil {
		m.chanMut.Lock()
		delete(m.channels, id)
		m.chanMut.Unlock()
		return nil, err
	}
	return ch, nil
}

var queueSize = 20

func (m *Multiplexer) routine() {
	for {
		select {
		case <-m.stop:
			return
		default:
		}

		nm, err := m.conn.Receive()
		if err != nil {
			slog.Print("connection to relay closed. stop.")
			return
		}
		rm, ok := nm.(*RelayMessage)
		if !ok {
			slog.Debug("multiplexer: received non relay message from ", m.conn.Remote())
			continue
		}
		m.chanMut.Lock()
		ch, ok := m.channels[rm.GetChannel()]
		if !ok {
			slog.Debug("multiplexer: received message for non saved signal: ", rm.Channel)
			m.chanMut.Unlock()
			continue
		}
		ch.dispatch(rm)
		m.chanMut.Unlock()
	}
}

func (m *Multiplexer) send(rm *RelayMessage) error {
	return m.conn.Send(rm)
}

func (m *Multiplexer) channelDone(id string) {
	m.chanMut.Lock()
	defer m.chanMut.Unlock()
	delete(m.channels, id)
	if len(m.channels) == 0 {
		m.conn.Close()
	}
}

type clientChannel struct {
	id     string
	m      *Multiplexer
	egress chan *RelayMessage
	stop   chan bool
	done   bool
	sync.Mutex
}

func newClientChannel(id string, m *Multiplexer) *clientChannel {
	return &clientChannel{
		id:     id,
		m:      m,
		egress: make(chan *RelayMessage, queueSize),
		stop:   make(chan bool, 1),
	}
}

var ErrClosed = errors.New("channel closed")

func (c *clientChannel) Receive() (string, []byte, error) {
	slog.Debugf("channel: waiting to receive message")
	select {
	case rm := <-c.egress:
		if rm.GetType() != RelayMessage_EGRESS {
			if rm.GetEgress() != nil {
				fmt.Println("BAD BAD BAD")
			}
			return "", nil, fmt.Errorf("channel %s: not egress receiving %d", c.id, rm.GetType())
		}
		eg := rm.GetEgress()
		slog.Debugf("channel: egress message received")
		return eg.GetAddress(), eg.GetBlob(), nil
	case <-c.stop:
		return "", nil, ErrClosed
	}
}

func (c *clientChannel) Close() {
	c.Lock()
	defer c.Unlock()
	if c.done {
		return
	}
	c.done = true
	if err := c.m.send(&RelayMessage{
		Channel: c.id,
		Type:    RelayMessage_LEAVE,
	}); err != nil {
		slog.Debugf("channel %s: %s", c.id, err)
	}
	close(c.stop)
	c.m.channelDone(c.id)
}

func (c *clientChannel) Send(blob []byte) error {
	return c.m.send(&RelayMessage{
		Channel: c.id,
		Type:    RelayMessage_INGRESS,
		Ingress: &Ingress{blob},
	})
}

var JoinTimeout = 1 * time.Minute

func (c *clientChannel) join() error {
	if err := c.m.send(&RelayMessage{
		Channel: c.id,
		Type:    RelayMessage_JOIN,
	}); err != nil {
		slog.Debug("could not join: ", err)
		return err
	}

	select {
	case mt := <-c.egress:
		if mt.GetType() != RelayMessage_JOIN_RESPONSE {
			return errors.New("signal received unexpected message")
		}
		jr := mt.GetJoinResponse()
		if jr.GetStatus() == JoinResponse_FAILURE {
			return errors.New("signal could not join: " + jr.GetReason())
		}
		return nil
	case <-time.After(JoinTimeout):
		return errors.New("join channel timed out")
	}
}

func (c *clientChannel) dispatch(rm *RelayMessage) {
	c.egress <- rm
}

func (c *clientChannel) Id() string {
	return c.id
}

// ReliableChannel is a Channel providing at least two guarantees:
// reliable delivery and ordering.
// For the moment, a very simple protocol is implemented: each packet
// going out is associated with a sequence number. The recipient must send back
// a positive ACK for that sequence number as soon as it receives the packet.
// On top of that, this reliable channel broadcasts a "discovery" packet saying
// "hey I'm there in this channel with address <....>". This allows to
// differentiate between peers in the same Channel. Of course, this does not provide
// any protections, it is just more easy to mitm this type of connection than a
// TCP connection but both are possible ;)
// A ReliableChannel has a interface definition similar to net.Listener. It
// expects a function so each time a new peer is recognized in the underlying
// Channel, a UnicastReliableChannel is created and dispatched.
/*type ReliableChannel interface {*/
//Channel
//Listen(func(UnicastReliableChannel))
//ActivePeers() []string
//}

// UnicastReliableChannel is an abstraction representing a
// pass-through-relay-through-channel one to one connection backed by the
// ReliableChannel.
/*type UnicastReliableChannel interface {*/
//Channel
//Peer() string
//}

/*type reliableChannel struct {*/
//Channel
//mut       sync.Mutex
//conns     map[string]*reliableConn
//localAddr string
//}

//func newReliableChannel(ch Channel, localAddr string) *reliableChannel {
//return &reliableChannel{
//Channel:   ch,
//conns:     make(map[string]reliableConn),
//localAddr: localAddr,
//}
//}

//func (r *reliableChannel) Listen(fn func(UnicastReliableChannel)) {
//for {
//from, buff, err := r.Receive()
//if err != nil {
//slog.Info("reliable: error receiving:", err)
//if err == ErrClosed {
//return
//}
//continue
//}

//mut.Lock()
//rc, ok := conns[from]
//// not yet seen peer
//if !ok {
//rc = newReliableConn(r.Channel, from, r.Id())
//conns[from] = rc
//go fn(rc)
//}

//msg, err := enc.Unmarshal(buff)
//if err != nil {
//slog.Debug("reliableconn: " + rc.peer + " error: " + err.Error())
//mut.Unlock()
//continue
//}
//rc.appendMsg(msg)
//mut.Unlock()
//}
//}

//func (r *reliableChannel) ActivePeers() []string {
//r.mut.Lock()
//defer r.mut.Unlock()
//keys := make([]string, 0, len(r.conns))
//for k := range r.conns {
//keys = append(keys, k)
//}
//return keys
//}

//var Timeout = 30 * time.Second
//var MaxBackoffTimeout = 4
//var PingPeriod = 10 * time.Second

//type reliableConn struct {
//Channel
//peer     string
//pendings [][]byte
//lastMsg  time.Time
//sync.Mutex
//}

//func newReliableConn(ch Channel, from string) *reliableConn {
//return &reliableConn{
//peer:    from,
//id:      ch.Id(),
//lastMsg: time.Now(),
//Channel: ch,
//}
//}

//// appendMsg stores the given message to its internal queue and/or update the
//// last time it has received a message from that peer.
//func (rc *reliableConn) appendMsg(msg net.Message) {
//rc.Lock()
//defer rc.Unlock()
//rc.lastMsg = time.Now()

//switch reliable := msg.(type) {
//case *ReliablePing:
//rc.lastMsg = time.Now()
//case *ReliableMessage:
//rc.pendings = append(rc.pendings, buff)
//}
//}

//func (rc *reliableConn) ping() {
//for {
//select {
//case <-time.After(PingPeriod):
//rc.ping()
//case <-rc.closed:
//return
//}
//}
//}

//func (rc *reliableConn) watchdog() {
//backoff := 1
//for {
//select {
//case <-time.After(backoff * MaxTimeout):
//backoff += 1
//if backoff == MaxBackoffTimeout {
//break
//}
//case <-rc.newMessage:
//backoff = 1
//}
//}
//}

//func (rc *reliableConn) Close() {
//select {
//case _, o := <-rc.closed:
//if !o {
//return
//}
//default:
//}
//close(rc.closed)
//}

//func (rc *reliableConn) Receive() (string, []byte, error) {
//rc.Lock()
//defer rc.Unlock()
//if time.Since(rc.lastMsg) > MaxTimeout {
//return nil, nil, errors.New("reliableconn: " + rc.peer + " timeouted")
//}

//len = len(rc.pendings)
//first = rc.pendings[0]
//rc.pending[0] = rc.pendings[len-1]
//rc.pendings[len-1] = nil
//rc.pendings = rc.pendings[:len-1]
//return rc.peer, first, nil
//}

//func (rc *reliableConn) Send(buff []byte) error {
//return rc.Channel.Send(buff)
/*}*/
