package relay

import (
	"errors"
	"fmt"
	"time"

	net "github.com/nikkolasg/mulsigo/network"
	"github.com/nikkolasg/mulsigo/slog"
)

type Signal interface {
	// channel that receives messages that must be broadcasted as ingress
	// message to the relay
	Ingress() chan Ingress
	// channel that receives message that have been received from the relay as
	// Egress message,i.e. broadcasted messages.
	Egress() chan Egress
	Error() chan error
	Id() string
}

type Multiplexer struct {
	conn      net.Conn
	newSignal chan Signal
	delSignal chan Signal
}

func NewMultiplexer(c net.Conn) *Multiplexer {
	m := &Multiplexer{
		conn:      c,
		newSignal: make(chan Signal),
		delSignal: make(chan Signal),
	}
	go m.routine()
	return m
}

func (m *Multiplexer) AddSignal(s Signal) {
	m.newSignal <- s
}

func (m *Multiplexer) DelSignal(s Signal) {
	m.delSignal <- s
}

var queueSize = 20

func (m *Multiplexer) routine() {
	stopReceive := make(chan bool)
	signals := make(map[string]*signalInfo)
	relayMsg := make(chan RelayMessage, queueSize)

	go func() {
		for {
			nm, err := m.conn.Receive()
			if err != nil {
				slog.Info(err)
				continue
			}
			rm, ok := nm.(*RelayMessage)
			if !ok {
				slog.Debug("received non relay message from ", m.conn.Remote())
				continue
			}

			select {
			case <-stopReceive:
				return
			case relayMsg <- *rm:
			default:
			}
		}
	}()

	for {
		select {
		case s := <-m.newSignal:
			info := newSignalInfo(s, m.conn)
			signals[s.Id()] = info
			go info.routine()
		case s := <-m.delSignal:
			info, ok := signals[s.Id()]
			if !ok {
				continue
			}
			close(info.stop)
			delete(signals, s.Id())
		case egress := <-relayMsg:
			channel := egress.GetChannel()
			info, ok := signals[channel]
			if !ok {
				slog.Debug("received message for non saved signal")
				continue
			}
			info.egress <- egress
		}

	}
}

// manage the messages for one channel
type signalInfo struct {
	id     string            // id of the channel
	signal Signal            // the signal which to dispatch messages
	conn   net.Conn          // the connection to directly send raw message to relay
	egress chan RelayMessage // egress channel receiving messages from the multiplexer
	stop   chan bool         // signal to stop the processing
}

func newSignalInfo(s Signal, conn net.Conn) *signalInfo {
	return &signalInfo{
		id:     s.Id(),
		signal: s,
		conn:   conn,
		egress: make(chan RelayMessage, queueSize),
		stop:   make(chan bool, 1),
	}
}

func (s *signalInfo) routine() {
	if err := s.join(); err != nil {
		s.signal.Error() <- err
	}

	ingress := s.signal.Ingress()
	for {
		select {
		case msg := <-ingress:
			err := s.conn.Send(&RelayMessage{
				Channel: s.signal.Id(),
				Type:    RelayMessage_INGRESS,
				Ingress: &msg,
			})
			if err != nil {
				s.signal.Error() <- fmt.Errorf("signalInfo: %s", err.Error())
			}
		case msg := <-s.egress:
			if msg.GetType() != RelayMessage_EGRESS {

			}
		case <-s.stop:
			return
		}
	}
}

var JoinTimeout = 1 * time.Minute

func (s *signalInfo) join() error {
	if err := s.conn.Send(&RelayMessage{
		Channel: s.id,
		Type:    RelayMessage_JOIN,
	}); err != nil {
		return err
	}
	select {
	case mt := <-s.egress:
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

func (s *signalInfo) leave() {
	err := s.conn.Send(&RelayMessage{
		Channel: s.id,
		Type:    RelayMessage_LEAVE,
	})
	if err != nil {
		slog.Print("signalInfo: " + err.Error())
	}
}
