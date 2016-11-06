package lib

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPgpValidSig(t *testing.T) {
	pub, priv, err := NewKeyPair(nil)
	require.Nil(t, err)
	privScalar := priv.Scalar()

	var msg = []byte("Hello World")

	sig := SchnorrSign(privScalar, msg, nil)

	pubFile, err := os.Create("/tmp/public.pgp")
	require.Nil(t, err)
	sigFile, err := os.Create("/tmp/data.sig")
	require.Nil(t, err)
	dataFile, err := os.Create("/tmp/data")
	require.Nil(t, err)

	require.Nil(t, SerializePubKey(pubFile, pub[:], "test@test.test"))
	r := sig[:32]
	s := sig[32:]
	require.Nil(t, SerializeSignature(sigFile, msg, pub[:], r, s))

	dataFile.Write(msg)

	pubFile.Close()
	sigFile.Close()
	dataFile.Close()

	cmd := exec.Command("gpg2", "--homedir", "/tmp/", "--allow-non-selfsigned-uid", "--import", "/tmp/public.pgp")
	cmd.Stdout = os.Stdout
	require.Nil(t, cmd.Run())

	cmd = exec.Command("gpg2", "--homedir", "/tmp/", "--allow-non-selfsigned-uid", "--ignore-time-conflict", "--verify", "/tmp/data.sig")
	cmd.Stdout = os.Stdout
	require.Nil(t, cmd.Run())

	os.Remove(sigFile.Name())
	os.Remove(pubFile.Name())
	os.Remove(dataFile.Name())
}
