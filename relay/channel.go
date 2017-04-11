package relay

import (
	"github.com/dedis/onet/log"
	net "github.com/nikkolasg/mulsigo/network"
)

// channel holds a list of all participants registered to a channel designated
// by an ID. Each participant can broadcast message to a channel and receive
// message from others on the same channel.
type channel struct {
	id       string               // id of the channel
	relay    *Relay               // router to send messages
	incoming chan messageInfo     // incoming message coming from the router
	join     chan net.Address     // join message
	leave    chan net.Address     // leave message
	finished chan bool            // stop signal from the router
	clients  map[net.Address]bool // list of clients. Not concurrent safe.
}

// newChannel returns a new channel identified by "id". It launches the
// processing routine.
func newChannel(r *Relay, id string) *channel {
	ch := &channel{
		id:       id,
		incoming: make(chan messageInfo, ChannelQueueSize),
		join:     make(chan net.Address, ChannelSize),
		leave:    make(chan net.Address, ChannelSize),
		finished: make(chan bool, 1),
		relay:    r,
		clients:  make(map[net.Address]bool),
	}
	go ch.process()
	return ch
}

func (ch *channel) newMessage(from net.Address, incoming *RelayMessage) {
	switch incoming.GetType() {
	case RelayMessage_JOIN:
		ch.join <- from
	case RelayMessage_LEAVE:
		ch.leave <- from
	case RelayMessage_INGRESS:
		ch.incoming <- messageInfo{from, incoming.GetIngress()}
	default:
		log.Error("channel: unknown message type")
	}
}

func (ch *channel) process() {
	clients := ch.clients
	for {
		select {
		case <-ch.finished:
			return
		case addr := <-ch.join:
			log.Lvl2("channel", ch.id, ": adding client", addr.String())
			ch.addClient(addr)
		case addr := <-ch.leave:
			delete(clients, addr)
			if len(clients) == 0 {
				// delete this channel
				ch.relay.deleteChannel(ch.id)
				return
			}
		case info := <-ch.incoming:
			ingress := info.msg
			addr := info.address
			_, ok := clients[addr]
			if !ok {
				// unknown user
				log.Lvl2("channel: msg from unregistered user")
				continue
			}

			rm := &RelayMessage{
				Channel: ch.id,
				Egress: &Egress{
					Address: addr.String(),
					Blob:    ingress.GetBlob(),
				},
			}

			for c := range clients {
				if c == addr {
					continue
				}
				// XXX change to a more abstract way
				if err := ch.relay.router.Send(c, rm); err != nil {
					log.Errorf("channel %d: %s: %s", ch.id, c, err)
				}
			}
		}
	}
}

// addClient adds the client to the list of local clients if capacity is not
// exceeded and replay with a JOIN_ACK message.
func (ch *channel) addClient(client net.Address) {
	_, ok := ch.clients[client]
	if ok {
		// already registered user -> no op
		return
	}
	if len(ch.clients) < ChannelSize {
		ch.clients[client] = true
		return
	}
	jr := &JoinResponse{
		Status: JoinResponse_FAILURE,
		Reason: "channel: can't join a full channel",
	}
	log.Lvl2("adding client", client)
	if err := ch.relay.router.Send(client, &RelayMessage{
		Type:         RelayMessage_JOIN_RESPONSE,
		JoinResponse: jr,
	}); err != nil {
		log.Error(err)
	}
}

// stop makes the channel stop for processing messages.
func (ch *channel) stop() {
	close(ch.finished)
}

// messageInfo is a simple wrapper to wrap the sender of a message to the
// message in question.
type messageInfo struct {
	address net.Address
	msg     *Ingress
}
