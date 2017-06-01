package client

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPrivateIdentity(t *testing.T) {
	var name = "winston smith"
	priv, id, err := NewPrivateIdentity(name, rand.Reader)
	require.Nil(t, err)
	require.NotNil(t, priv)
	require.NotNil(t, id)

	pubCurveFromPriv := priv.PublicCurve25519()
	pubCurveFromPub := id.PublicCurve25519()
	require.Equal(t, pubCurveFromPub, pubCurveFromPriv)

}
