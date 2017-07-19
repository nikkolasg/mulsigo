package client

import "gopkg.in/dedis/kyber.v1/share/pedersen/dkg"

const (
	PROTOCOL_PDKG = iota
	PROTOCOL_RDKG = iota
)

type ClientMessage struct {
	Type uint32

	PDkg *PDkg
}

type PDkg struct {
	Deal          *dkg.Deal
	Response      *dkg.Response
	Justification *dkg.Justification
}

type ReliableType bool

const (
	RELIABLE_DATA ReliableType = true
	RELIABLE_ACK  ReliableType = false
)

type ReliablePacket struct {
	Type     ReliableType
	Sequence uint32
	Data     []byte `protobuf:"opt"`
}
