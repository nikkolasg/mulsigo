package client

import "github.com/dedis/crypto/ed25519"

var Suite = ed25519.NewAES128SHA256Ed25519(false)

// Client can performs all distributed operations such as:
// - generating a random distributed secret (rds)
// - with two rds, it can sign a message
// 		+ a self key signing process to be pgp compatible
//		+ a regular signing process to sign any message in a pgp compatible way
type Client struct {
}

// Leader is a Client that can performs the initial operations such as sending
// the first message containing the public informations of the pgp key (name,
// email etc), sending the file/text to sign.
type Leader struct {
	*Client
}

func NewLeader() *Leader {
	panic("not implemented")
}

func NewClient() *Client {
	panic("not implemented")
}
