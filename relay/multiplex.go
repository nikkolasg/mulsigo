package relay

import (
	"errors"
	"fmt"
	"sync"
	"time"

	net "github.com/nikkolasg/mulsigo/network"
	"github.com/nikkolasg/mulsigo/slog"
)

type Channel interface {
	Send([]byte) error
	Receive() (*Egress, error)
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
	slog.Debugf("multiplexer: new channel %s joining...", id)
	m.chanMut.Lock()
	if s, ok := m.channels[id]; ok {
		m.chanMut.Unlock()
		return s, nil
	}
	slog.Debugf("multiplexer: new channel %s joining...", id)
	ch := newClientChannel(id, m)
	m.channels[id] = ch
	m.chanMut.Unlock()

	err := ch.join()
	slog.Debugf("multiplexer: new channel %s joined!", id)
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
}

type clientChannel struct {
	id     string
	m      *Multiplexer
	egress chan *RelayMessage
	stop   chan bool
}

func newClientChannel(id string, m *Multiplexer) *clientChannel {
	return &clientChannel{
		id:     id,
		m:      m,
		egress: make(chan *RelayMessage, queueSize),
		stop:   make(chan bool, 1),
	}
}

func (c *clientChannel) Receive() (*Egress, error) {
	select {
	case rm := <-c.egress:
		if rm.GetType() != RelayMessage_EGRESS {
			if rm.GetEgress() != nil {
				fmt.Println("BAD BAD BAD")
			}
			return nil, fmt.Errorf("channel %s: not egress receiving %d", c.id, rm.GetType())
		}
		eg := rm.GetEgress()
		return eg, nil
	case <-c.stop:
		return nil, fmt.Errorf("channel %s: closed")
	}
}

func (c *clientChannel) Close() {
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
	slog.Debug("clientChannel: dispatching relay message...")
	c.egress <- rm
}

func (c *clientChannel) Id() string {
	return c.id
}
