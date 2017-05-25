package client

import (
	"bytes"
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/dedis/crypto/random"
	net "github.com/nikkolasg/mulsigo/network"
	"github.com/nikkolasg/mulsigo/util"
	"gopkg.in/dedis/crypto.v0/abstract"
)

type Private struct {
	Key abstract.Scalar
}

type Public struct {
	Key abstract.Point
}

func NewPrivateIdentity(name string) (*Private, *Identity) {
	sig := SigSuite.NewKey(random.Stream)
	priv := &Private{
		Key: sig,
	}
	sigPub := SigSuite.Point().Mul(nil, sig)
	id := &Identity{
		Name: name,
		Key:  sigPub,
	}
	return priv, id
}

type privateToml struct {
	Key string
}

func (p *Private) Toml() interface{} {
	sigStr, _ := util.ScalarToString64(p.Key)
	return &privateToml{sigStr}
}

func (p *Private) FromToml(f string) error {
	pt := &privateToml{}
	_, err := toml.Decode(f, pt)
	if err != nil {
		return err
	}
	p.Key, err = util.String64ToScalar(SigSuite, pt.Key)
	return err
}

type Identity struct {
	Name string
	Key  abstract.Point
	// reachable - usually empty if using a relay but if provided, will enable
	// one to make direct connection between a pair of peers.
	Address net.Address
}

func (i *Identity) Repr() string {
	var buff bytes.Buffer
	str, _ := util.PointToString64(i.Key)
	fmt.Fprintf(&buff, "id:\t%s ", i.Name)
	fmt.Fprintf(&buff, "\n\t%s", str)
	return buff.String()
}

type identityToml struct {
	Name    string
	Key     string
	Address string
}

func (i *Identity) Toml() interface{} {
	sigStr, _ := util.PointToString64(i.Key)
	return &identityToml{
		Key:     sigStr,
		Name:    i.Name,
		Address: i.Address.String(),
	}
}

func (i *Identity) FromToml(f string) error {
	it := &identityToml{}
	_, err := toml.Decode(f, it)
	if err != nil {
		return err
	}
	sigPub, err := util.String64ToPoint(SigSuite, it.Key)
	if err != nil {
		return err
	}
	i.Name = it.Name
	i.Key = sigPub
	i.Address = net.Address(it.Address)
	return nil
}

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
