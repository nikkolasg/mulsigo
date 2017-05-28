package client

import (
	"bytes"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/BurntSushi/toml"
	net "github.com/nikkolasg/mulsigo/network"
	"github.com/nikkolasg/mulsigo/util"
	"gopkg.in/dedis/crypto.v0/abstract"
	"gopkg.in/dedis/crypto.v0/sign"
)

type Private struct {
	seed []byte
}

func (p *Private) Scalar() abstract.Scalar {
	h := sha512.New()
	h.Write(p.seed[:32])
	digest := h.Sum(nil)

	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64

	s := suite.Scalar()
	s.SetBytes(digest)
	return s
}

func (p *Private) Public() Public {
	s := p.Scalar()
	return Suite.Point().Mul(nil, s)
}

type Public abstract.Point

func NewPrivateIdentity(name string, r io.Reader) (*Private, *Identity) {
	// only need the first 32 bytes
	var seed [32]byte
	_, err := io.ReadFull(r, seed)
	if err != nil {
		return nil, nil, err
	}
	priv := &Private{seed}

	id := &Identity{
		Name:      name,
		CreatedAt: time.Now().Unix(),
		Key:       priv.Public(),
	}

	err := id.selfsign(priv, r)
	return priv, id, err
}

type Identity struct {
	Name      string
	Key       Public
	CreatedAt int64
	Signature []byte
	// reachable - usually empty if using a relay but if provided, will enable
	// one to make direct connection between a pair of peers.
	Address net.Address
}

// selfsign marshals the identity's name,creation time and public key and then
// signs the resulting buffer. The signature can be accessed through the
// Signature field of the Identity. It is a Schnorr signature that can be
// verified using Ed25519 (first EdDSA versions) signature verification
// routines.
func (i *Identity) selfsign(p *Private, r io.Reader) error {
	var buff bytes.Buffer
	buff.WriteString(i.Name)
	err := binary.Write(&buff, binary.LittleEndian, i.CreatedAt)
	if err != nil {
		return err
	}
	b, err := abstract.Point(i.Key).MarshalBinary()
	if err != nil {
		return err
	}
	buff.Write(b)
	i.Signature, err = sign.Schnorr(Suite, p.Scalar(), buff.Bytes())
	return err
}

func (i *Identity) Repr() string {
	var buff bytes.Buffer
	str, _ := util.PointToString64(i.Key)
	fmt.Fprintf(&buff, "id:\t%s ", i.Name)
	fmt.Fprintf(&buff, "\n\t%s", str)
	return buff.String()
}

type privateToml struct {
	Seed string
}

func (p *Private) Toml() interface{} {
	seedStr, _ := util.ScalarToString64(p.seed)
	return &privateToml{seedStr}
}

func (p *Private) FromToml(f string) error {
	pt := &privateToml{}
	_, err := toml.Decode(f, pt)
	if err != nil {
		return err
	}
	p.seed, err = util.String64ToScalar(Suite, pt.Seed)
	return err
}

type identityToml struct {
	Name      string
	Key       string
	CreatedAt int64
	Signature string
	Address   string
}

func (i *Identity) Toml() interface{} {
	publicStr, _ := util.PointToStringHex(Suite, i.Key)
	sigStr := hex.EncodeToString(i.Signature)
	return &identityToml{
		Key:       publicStr,
		Signature: sigStr,
		Name:      i.Name,
		Address:   i.Address.String(),
		CreatedAt: i.CreatedAt,
	}
}

func (i *Identity) FromToml(f string) error {
	it := &identityToml{}
	_, err := toml.Decode(f, it)
	if err != nil {
		return err
	}
	public, err := util.StringHexToPoint(Suite, it.Key)
	if err != nil {
		return err
	}
	signature, err := hex.DecodeString(it.Signature)
	if err != nil {
		return err
	}
	i.Name = it.Name
	i.Key = public
	i.Address = net.Address(it.Address)
	return nil
}

// GroupConfig is the public configuration of a group using mulsigo. It is
// similar to a public pgp identity with a name, email and comment. It contains
// the additional public information on all the participants of the group.
type GroupConfig struct {
	Name    string
	Email   string
	Comment string

	// coefficients of the public polynomial
	Public []abstract.Point
	// list of node's identity participating
	Ids []Identity
	// threshold of the group
	T int

	PgpID     uint64
	PgpPublic string
}

type groupConfigToml struct {
	Name    string
	Email   string
	Comment string

	// coefficient of the public polynomial
	Public []string
	// list of node's identity participating
	Ids []identityToml
	// threshold of the group
	T int

	PgpID     string
	PgpPublic string
}

func (g *GroupConfig) toml() interface{} {
	publics := make([]string, len(g.Public))
	for i, p := range g.Public {
		s, err := util.PointToString64(p)
		if err != nil {
			return err
		}
		publics[i] = s
	}

	ids := make([]identityToml, len(g.Ids))
	for i, id := range g.Ids {
		itoml := id.Toml().(*identityToml)
		ids[i] = *itoml
	}

	return &groupConfigToml{
		Name:      g.Name,
		Email:     g.Email,
		Comment:   g.Comment,
		Public:    publics,
		Ids:       ids,
		T:         g.T,
		PgpPublic: g.PgpPublic,
	}
}
