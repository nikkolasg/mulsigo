package relay

import (
	"github.com/dedis/onet/log"
	net "github.com/dedis/onet/network"
)

type client struct {
	conn    net.Conn
	address string
	relay   *Relay
	out     chan ChannelOutgoingMessage
}

func newClient(c net.Conn, relay *Relay) *client {
	client := &client{
		conn:    c,
		out:     make(chan ChannelOutgoingMessage, ChannelQueueSize),
		address: c.Remote().String(),
	}
	return client
}

func (c *client) readMessages() {
	conn := c.conn
	defer func() {
		c.relay.unregisterClient(c)
		c.conn.Close()
	}()
	r := c.relay
	for {
		env, err := conn.Receive()
		if err != nil {
			log.Errorf("relay: conn %s: err %s", c.address, err)
			continue
		}
		if env.MsgType != RelayMessageType {
			log.Errorf("relay: conn %s: wrong message type", c.address)
			continue
		}
		msg := env.Msg.(*RelayMessage)
		switch {
		case msg.Msg != nil:
			r.dispatchToChannel(c, msg.Msg)
		case msg.Join != nil:
			r.joinChannel(c, msg.Join)
		case msg.Leave != nil:
			r.leaveChannel(c, msg.Leave)
		default:
			log.Errorf("relay: weird message from %s", c.address)
		}
	}
}

func (c *client) writeMessages() {
	for msg := range c.out {
		if err := c.conn.Send(&msg); err != nil {
			c.relay.unregisterClient(c)
			c.conn.Close()
			break
		}
	}
}
