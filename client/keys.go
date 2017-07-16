package client

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/ed25519"

	"github.com/BurntSushi/toml"
	"github.com/agl/ed25519/extra25519"
	net "github.com/nikkolasg/mulsigo/network"
	kyber "gopkg.in/dedis/kyber.v1"
	"gopkg.in/dedis/kyber.v1/group/edwards25519"
	"gopkg.in/dedis/kyber.v1/util/encoding"
)

var Group = edwards25519.NewAES128SHA256Ed25519(false)

type Private struct {
	seed *ed25519.PrivateKey
}

func (p *Private) Scalar() kyber.Scalar {
	h := sha512.New()
	h.Write((*p.seed)[:32])
	digest := h.Sum(nil)

	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64

	s := Group.Scalar()
	s.SetBytes(digest)
	return s
}

func (p *Private) Public() []byte {
	return (*p.seed)[32:]
}

func (p *Private) PrivateCurve25519() [32]byte {
	return ed25519PrivateToCurve25519(p.seed)
}

func (p *Private) PublicCurve25519() [32]byte {
	priv := p.PrivateCurve25519()
	var pubCurve [32]byte
	curve25519.ScalarBaseMult(&pubCurve, &priv)

	return pubCurve
}

func NewPrivateIdentity(name string, r io.Reader) (*Private, *Identity, error) {
	pub, privEd, err := ed25519.GenerateKey(r)
	priv := &Private{&privEd}

	id := &Identity{
		Name:      name,
		CreatedAt: time.Now().Unix(),
		Key:       pub,
	}

	err = id.selfsign(priv, r)
	return priv, id, err
}

type Identity struct {
	Name      string
	Key       []byte
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
	buff.Write(i.Key)
	i.Signature = ed25519.Sign(*p.seed, buff.Bytes())
	return nil
}

func (i *Identity) Repr() string {
	var buff bytes.Buffer
	str := base64.StdEncoding.EncodeToString(i.Key)
	fmt.Fprintf(&buff, "id:\t%s ", i.Name)
	fmt.Fprintf(&buff, "\n\t%s", str)
	return buff.String()
}

func (i *Identity) ID() string {
	return base64.StdEncoding.EncodeToString(i.Key)
}

func (i *Identity) PublicCurve25519() [32]byte {
	var pubEd25519 [32]byte
	var pubCurve [32]byte
	copy(pubEd25519[:], i.Key)
	ret := extra25519.PublicKeyToCurve25519(&pubCurve, &pubEd25519)
	if !ret {
		panic("corrupted private key? can't convert to curve25519")
	}
	return pubCurve
}

type privateToml struct {
	Seed string
}

func (p *Private) Toml() interface{} {
	seedStr := base64.StdEncoding.EncodeToString(*p.seed)

	return &privateToml{seedStr}
}

func (p *Private) FromToml(f string) error {
	pt := &privateToml{}
	_, err := toml.Decode(f, pt)
	if err != nil {
		return err
	}
	seed, err := base64.StdEncoding.DecodeString(pt.Seed)
	seedEd25519 := ed25519.PrivateKey(seed)
	p.seed = &seedEd25519
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
	publicStr := base64.StdEncoding.EncodeToString(i.Key)
	sigStr := base64.StdEncoding.EncodeToString(i.Signature)
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
	public, err := base64.StdEncoding.DecodeString(it.Key)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(it.Signature)
	if err != nil {
		return err
	}
	i.Name = it.Name
	i.Key = public
	i.Address = net.Address(it.Address)
	i.Signature = signature
	return nil
}

func (i *Identity) Point() kyber.Point {
	p := Group.Point()
	if err := p.UnmarshalBinary(i.Key); err != nil {
		panic(err)
	}
	return p
}

// GroupConfig is the public configuration of a group using mulsigo. It is
// similar to a public pgp identity with a name, email and comment. It contains
// the additional public information on all the participants of the group.
type GroupConfig struct {
	Name    string
	Email   string
	Comment string

	// coefficients of the public polynomial
	Public []kyber.Point
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
		s, err := encoding.PointToString64(Group, p)
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

func ed25519PrivateToCurve25519(p *ed25519.PrivateKey) [32]byte {
	var buff [64]byte
	copy(buff[:], *p)
	var curvePriv [32]byte

	extra25519.PrivateKeyToCurve25519(&curvePriv, &buff)
	return curvePriv
}

func ed25519PublicToCurve25519(p *ed25519.PublicKey) ([32]byte, bool) {
	var buff [32]byte
	copy(buff[:], *p)
	var curvePub [32]byte

	ret := extra25519.PublicKeyToCurve25519(&curvePub, &buff)
	return curvePub, ret
}
