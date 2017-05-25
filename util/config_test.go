package util

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/dedis/crypto/abstract"
	"github.com/stretchr/testify/assert"
)

type simple struct {
	Val int
}

type complex struct {
	Val abstract.Point
}

type complexToml struct {
	Val string
}

func (c *complex) toml() interface{} {
	b, _ := c.Val.MarshalBinary()
	s := base64.StdEncoding.EncodeToString(b)
	return &complexToml{s}
}

func (c *complex) fromToml(str string) error {
	ct := &complexToml{}
	if _, err := toml.Decode(str, ct); err != nil {
		return err
	}
	buff, err := base64.StdEncoding.DecodeString(ct.Val)
	if err != nil {
		return err
	}
	p := s.Point()
	if err := p.UnmarshalBinary(buff); err != nil {
		return err
	}
	c.Val = p
	return nil
}

func TestConfigReadWrite(t *testing.T) {
	tmp := os.TempDir()
	defer os.RemoveAll(tmp)
	c := NewConfigWithPath(tmp)

	simp := &simple{10}
	comp := &complex{s.Point().Base()}

	assert.Nil(t, c.Write("simple", simp))
	s2 := &simple{}
	assert.Nil(t, c.Read("simple", s2))
	assert.Equal(t, simp, s2)
	assert.Error(t, c.Read("simple2", s2))

	assert.Nil(t, c.Write("complex", comp))
	comp2 := &complex{}
	assert.Nil(t, c.Read("complex", comp2))
	assert.Equal(t, comp.Val.String(), comp2.Val.String())

	path := filepath.Join(c.Path(), "complex")
	comp3 := &complexToml{}
	_, err := toml.DecodeFile(path, comp3)
	assert.Nil(t, err)

	_, err = toml.DecodeFile(path, comp)
	assert.Error(t, err)

}
