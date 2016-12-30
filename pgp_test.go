package main

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"

	"github.com/stretchr/testify/require"
)

var name = "TestName"
var email = "TestEmail"
var comment = "TestComment"

func TestGpgCreateAndParseEd25519(t *testing.T) {
}

func TestPgpGolangRSASig(t *testing.T) {
	dirName, err := ioutil.TempDir("", "mulsigo")
	defer os.RemoveAll(dirName)
	require.Nil(t, err)
	var pubFileName = path.Join(dirName, "pub.pgp")
	var privFileName = path.Join(dirName, "priv.pgp")
	var dataFileName = path.Join(dirName, "data")
	var sigFileName = dataFileName + ".sig"

	pubFile, err := os.Create(pubFileName)
	defer pubFile.Close()
	require.Nil(t, err)

	privFile, err := os.Create(privFileName)
	defer privFile.Close()
	require.Nil(t, err)

	dataFile, err := os.Create(dataFileName)
	defer dataFile.Close()
	require.Nil(t, err)

	sigFile, err := os.Create(sigFileName)
	defer sigFile.Close()
	require.Nil(t, err)

	_seed := "Hello World, I'm gonna be your seed during this test, would you?"
	var seed = bytes.NewBuffer([]byte(_seed))

	config := packet.Config{
		Rand: seed,
	}
	ent, err := openpgp.NewEDDSAEntity(name, email, comment, &config)
	require.Nil(t, err)

	p := ent.PrimaryKey.PublicKey.(*ed25519.PublicKey)
	fmt.Println("Public key created:", hex.EncodeToString([]byte(*p)))
	var id string
	var identity *openpgp.Identity
	for name, i := range ent.Identities {
		if id == "" {
			id = name
			identity = i
		}
	}

	err = identity.SelfSignature.SignUserId(id, ent.PrimaryKey, ent.PrivateKey, nil)
	require.Nil(t, err)

	err = ent.Serialize(pubFile)
	require.Nil(t, err)

	gpg2Import(dirName, pubFileName, t)

	/*err = ent.SerializePrivate(privFile, nil)*/
	//require.Nil(t, err)

	//gpg2Import(dirName, privFileName, t)

	sig := &packet.Signature{
		PubKeyAlgo: packet.PubKeyAlgoEDDSA,
		Hash:       crypto.SHA256,
	}

	sig.IssuerKeyId = &ent.PrimaryKey.KeyId
	msg := []byte("Hello World!")

	h := sha256.New()
	_, err = h.Write(msg)
	require.Nil(t, err)

	if err := sig.Sign(h, ent.PrivateKey, nil); err != nil {
		t.Fatal(err)
	}

	h = sha256.New()
	_, err = h.Write(msg)
	require.Nil(t, err)

	if err := ent.PrivateKey.VerifySignature(h, sig); err != nil {
		t.Fatal(err)
	}

	err = sig.Serialize(sigFile)
	require.Nil(t, err)

	_, err = dataFile.Write(msg)
	require.Nil(t, err)

	var out bytes.Buffer
	cmd := exec.Command("gpg2", "--homedir", dirName, "--verify", sigFileName)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	fmt.Println(cmd.Args)
	fmt.Println(out.String())
	require.Nil(t, err)

}

func gpg2Import(dirName, fileName string, t *testing.T) {
	var out bytes.Buffer
	cmd := exec.Command("gpg2", "--homedir", dirName, "--import", fileName)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	fmt.Println(cmd.Args)
	fmt.Println(out.String())
	require.Nil(t, err)
}

/*func TestPgpGolangSig(t *testing.T) {*/
//dirName, err := ioutil.TempDir("", "mulsigo")
//defer os.RemoveAll(dirName)
//require.Nil(t, err)
//var pubFileName = path.Join(dirName, "pub.pgp")
//fmt.Println("Dirname: ", dirName)

//pubFile, err := os.Create(pubFileName)
//require.Nil(t, err)

//var currentTime = time.Now().Add(-20 * time.Hour)

//_seed := "Hello World, I'm gonna be your seed during this test, would you?"
//var seed = bytes.NewBuffer([]byte(_seed))
//eddsaPub, eddsaPriv, err := ed25519.GenerateKey(seed)
//pointer := ed25519.PrivateKey(eddsaPriv)

//eddsaPrivateKey := packet.NewEDDSAPrivateKey(time.Now(), &pointer)
//pubCasted := ed25519.PublicKey([]byte(eddsaPub[:]))
//var pgpPub = packet.NewEDDSAPublicKey(currentTime, &pubCasted)

//uid := packet.NewUserId(name, email, comment)
//require.NotNil(t, uid)

//primary := true

//SelfSignature := &packet.Signature{
//CreationTime: currentTime,
//SigType:      packet.SigTypePositiveCert,
//PubKeyAlgo:   packet.PubKeyAlgoRSA,
//Hash:         crypto.SHA256,
//IsPrimaryId:  &primary,
//FlagsValid:   true,
//FlagSign:     true,
//FlagCertify:  true,
//IssuerKeyId:  &pgpPub.KeyId,
//}
//require.Nil(t, SelfSignature.SignUserId(uid.Id, pgpPub, eddsaPrivateKey, nil))

//// serialize
//require.Nil(t, pgpPub.Serialize(pubFile))
//require.Nil(t, uid.Serialize(pubFile))
//require.Nil(t, SelfSignature.Serialize(pubFile))

//require.Nil(t, pubFile.Close())
//var out bytes.Buffer
//cmd := exec.Command("gpg2", "--homedir", dirName, "--import", pubFileName)
//cmd.Stdout = &out
//cmd.Stderr = &out
//err = cmd.Run()
//fmt.Println(cmd.Args)
//fmt.Println(out.String())
//require.Nil(t, err)
//}

//func TestPgpValidSig(t *testing.T) {
//dirName, err := ioutil.TempDir("", "mulsigo")
//defer os.RemoveAll(dirName)
//require.Nil(t, err)
//var pubFileName = path.Join(dirName, "pub.pgp")
//var dataFileName = path.Join(dirName, "data")
//var sigFileName = path.Join(dirName, "data.sig")
//fmt.Println("Dirname: ", dirName)

//pub, priv, err := NewKeyPair(nil)
//require.Nil(t, err)
//privScalar := priv.Scalar()

//var msg = []byte("Hello World")
//h := sha256.New()
//HashMessage(h, msg)
//var preHashed = h.Sum(nil)
//sig := SchnorrSign(privScalar, preHashed, nil)

//require.True(t, SchnorrVerify(pub, preHashed, sig))

//pubFile, err := os.Create(pubFileName)
//require.Nil(t, err)
//sigFile, err := os.Create(sigFileName)
//require.Nil(t, err)
//dataFile, err := os.Create(dataFileName)
//require.Nil(t, err)

//pubCasted := ed25519.PublicKey([]byte(pub[:]))
//var pgpPub = packet.NewEDDSAPublicKey(time.Now().Add(-20*time.Hour), &pubCasted)

//pgpPub.Serialize(pubFile)

//require.Nil(t, SerializePubKey(pubFile, pub[:], "test@test.test"))
//r := sig[:32]
//s := sig[32:]
//require.Nil(t, SerializeSignature(sigFile, msg, pub[:], r, s))

//dataFile.Write(msg)

//pubFile.Close()
//sigFile.Close()
//dataFile.Close()

//cmd := exec.Command("gpg2", "--homedir", dirName, "--allow-non-selfsigned-uid", "--import", pubFileName)
//cmd.Stdout = os.Stdout
//require.Nil(t, cmd.Run())

//cmd = exec.Command("gpg2", "--homedir", dirName, "--allow-non-selfsigned-uid", "--ignore-time-conflict", "--verify", sigFileName)
//cmd.Stdout = os.Stdout
//require.Nil(t, cmd.Run())

////cmd = exec.Command("rm", "-rf", dirName)
////cmd.Run()
//}

// exists returns whether the given file or directory exists or not
func path_exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}
