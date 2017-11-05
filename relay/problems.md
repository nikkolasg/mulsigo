# problem

Since connection are not direct, there is for the moment no way to know whether
a message sent has been received by anybody on a channel, like TCP provides.

1) The Noise handshake client Hello does not know if it has been received so
handshake may never finish in limited time

2) The protocol above don't know when the other party has left, connection drop
out etc

## Higher level problem: 

This problem is related to the liveness/availability/DOS problem. Any attacker can 
1) control the relay and do whatever it wants (delete messages, delay ACK, etc)
    -> no solution to that: change relay or go to full point to point
    communication
2) Attack the relay by spamming any channels it can find. For example if the 
attacker has knowledge of the public keys involved for the moment it can spam
the channel and potentially DOS the application.

In the direct communication setting the way to DOS the application in 1) is to be in a 
MitM position. In the relay setup, the cost is much less for 1).
Not sure this is something I want to tackle now however. 

# possible solutions

## relay keeping messages

Channel keeps each messages in memory for 10mn and distribute them when a
new server joins the channel. When timeout occurs, if messages hos not been
distributed:
    - reply to the author, "not reached" ?
    - just drop the message

Pros: know when a message have been reached to "someone"
Cons: complexity, memory problem, out of context messages etc

## middleware channel implementation wth the ACK mechanism

Creating a ACKChannel implementing some  kind of ACK mechanism: for each message
sent, expect an ACK. It must keep track of who's sending back an ACK.
Noise would have to try multiple times for each address. 
Pros: direct way of knowing a message's impact => return error if no receiver
Cons: no way to know if the right recipient received the message. receiving an
ACK may come from an attacker instead of the right recipient.

## Channel as a LISTENER 

Each new person joining a channel sends (periodically?) an advertisement message "I'm here using address <...> (with pubkey <...>)"
A channel receiving such messages creates a "connection" if the author is seen
for the first time. Maintaining a connection requires the channel to see the
advertisement message every 30sec or so. Otherwise it drops.
Pros: multiplexing inside channel into individual connection => easier to reason
with for higher level protocols. If combined with ACK mechanism, direct + 
individualized error 
Cons: no direct failure mechanisms, only after ACKPeriod timeout

# Design:

The relay code is kept the same: join a channel "room" broadcasts in it and
leave.

The client relay code will be as such:

client.Conn => multiplexer  => channel listen() -> Conn
                                                -> Conn
                            => channel
                            => ...

For each Conn, try out a noiseStream over it. If it works, just drop all others
(futures) conns

type ReliableChannel interface {
    Send() 
    Receive()
    Close()
}

type ChannelListener interface {
    Listen(func(ChannelConn))
    Close()
}

// transform from Channel to ChanneListener
func (c *Channel) ChannelListener() ChannelListener { }

//
func (c *ChannelListener) maintenance() {
    For each message:
        switch type:
            case ACKMessage:
                if message.Author is firstTime:
                    c.launchNewConn()
                else:
                    // no op, keep connection

            case Message:
                c.dispatchToConn()

 // AND watch every ACK
 for every 10 sec {
    for all connections c {
        if time.Now() -  c.lastACK  < 30 sec {
            c.close()
        }
    }
 }
}
