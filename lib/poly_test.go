package lib

import (
	"testing"

	"github.com/alecthomas/assert"
	"github.com/stretchr/testify/require"
)

func TestPolyShare(t *testing.T) {
	var k uint32 = 5
	var n uint32 = 10
	_, priv, err := NewKeyPair(nil)
	require.Nil(t, err)

	secret := priv.Scalar()
	poly, err := NewPoly(nil, priv, k)
	require.Nil(t, err)

	shares := make([]Share, k)
	for i := 0; i < int(k); i++ {
		shares[i] = poly.Share(uint32(i))
	}

	recons, err := Reconstruct(shares, k, n)
	assert.Nil(t, err)
	assert.True(t, secret.Equal(recons.Int))
}
