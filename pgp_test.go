package lib

import (
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"

	"github.com/stretchr/testify/require"
)

func TestPgpGolangSig(t *testing.T) {

}

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

	pubCasted := ed25519.PublicKey([]byte(pub[:]))
	var pgpPub = packet.NewEDDSAPublicKey(time.Now().Add(-20*time.Hour), &pubCasted)

	entity, err := openpgp.NewEntity("Test Entity", " <yep> ", nil)
	require.Nil(t, err)

	pgpPub.Serialize(pubFile)

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
