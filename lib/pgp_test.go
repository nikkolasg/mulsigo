package lib

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPgpValidSig(t *testing.T) {
	dirName, err := ioutil.TempDir("", "mulsigo")
	defer os.RemoveAll(dirName)
	require.Nil(t, err)
	var pubFileName = path.Join(dirName, "pub.pgp")
	var dataFileName = path.Join(dirName, "data")
	var sigFileName = path.Join(dirName, "data.sig")
	fmt.Println("Dirname: ", dirName)

	pub, priv, err := NewKeyPair(nil)
	require.Nil(t, err)
	privScalar := priv.Scalar()

	var msg = []byte("Hello World")
	h := sha256.New()
	HashMessage(h, msg)
	var preHashed = h.Sum(nil)
	sig := SchnorrSign(privScalar, preHashed, nil)

	require.True(t, SchnorrVerify(pub, preHashed, sig))

	pubFile, err := os.Create(pubFileName)
	require.Nil(t, err)
	sigFile, err := os.Create(sigFileName)
	require.Nil(t, err)
	dataFile, err := os.Create(dataFileName)
	require.Nil(t, err)

	require.Nil(t, SerializePubKey(pubFile, pub[:], "test@test.test"))
	r := sig[:32]
	s := sig[32:]
	require.Nil(t, SerializeSignature(sigFile, msg, pub[:], r, s))

	dataFile.Write(msg)

	pubFile.Close()
	sigFile.Close()
	dataFile.Close()

	cmd := exec.Command("gpg2", "--homedir", dirName, "--allow-non-selfsigned-uid", "--import", pubFileName)
	cmd.Stdout = os.Stdout
	require.Nil(t, cmd.Run())

	cmd = exec.Command("gpg2", "--homedir", dirName, "--allow-non-selfsigned-uid", "--ignore-time-conflict", "--verify", sigFileName)
	cmd.Stdout = os.Stdout
	require.Nil(t, cmd.Run())

	//cmd = exec.Command("rm", "-rf", dirName)
	//cmd.Run()
}

// exists returns whether the given file or directory exists or not
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}
