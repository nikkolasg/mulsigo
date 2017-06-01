package pgp

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/ed25519"

	"github.com/agl/ed25519/extra25519"
	"github.com/flynn/noise"
	"github.com/stretchr/testify/require"
)

var tmpDir string

func TestMain(m *testing.M) {
	var err error
	tmpDir, err = ioutil.TempDir("", "")
	if err != nil {
		fmt.Print("Fatal error with temp dir: ", err)
		os.Exit(1)
	}
	fmt.Println("Running with tmpDir = ", tmpDir)
	ret := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(ret)
}

type eddsaSigner struct {
	priv *ed25519.PrivateKey
}

func (s *eddsaSigner) Public() crypto.PublicKey {
	return ed25519.PublicKey(*s.priv)[32:]
}

func (s *eddsaSigner) Sign(rand io.Reader, msg []byte, opts crypto.SignerOpts) ([]byte, error) {
	return s.priv.Sign(rand, msg, opts)
}

var batch = `%no-protection
Key-Type: eddsa
Key-Curve: Ed25519
Name-Real: Joe Tester
Name-Comment: with stupid passphrase
Name-Email: joe@foo.bar
Expire-Date: 1m
# Do a commit here, so that we can later print "done" :-)
%commit
%echo done
`

// generated with gpg --homdir test --expert --full-gen-key --batch foo
var fakeKey = "9458045864343316092B06010401DA470F010107404BBEBF9709835BC984D4206A95A6D83AE70D0A89AE9F2AB3F0A8520CDEDDA6100000FF78FA01D6275B2CBAF194E8604D0224EAC48BA35377271A580297D75E96B03D520FD2B4177465737461203C74657374614074657374612E636F6D3E889604131608003E16210444CB29D2EB4D725FFA38F5BF0C7451265AF77437050258643433021B03050900278D00050B09080702061508090A0B020416020301021E01021780000A09100C7451265AF77437BA250100F6093F29BA030E5E38FB0731221608985D7465E4877942A8F234E9450894769601009DBB4BDE9596E25CDBA3E5149E258F64B46396D3459AA8A6B43E8CD159782208"

var info = &GpgKeyInfo{"Strangelove", "love the bomb", "nob@bon.onb"}

func TestGpgCreateKey(t *testing.T) {
	out, err := createKey(tmpDir, info)
	require.Nil(t, err)
	require.Contains(t, string(out), "mulsigo")

	d, err := os.Open(tmpDir)
	require.Nil(t, err)
	defer d.Close()
	names, err := d.Readdirnames(-1)
	require.Nil(t, err)
	var found bool
	for _, name := range names {
		if strings.Contains(name, "private-keys") {
			found = true
		}
	}
	require.True(t, found)
}

func TestGpgReadKey(t *testing.T) {
	_, err := createKey(tmpDir, info)
	require.Nil(t, err)

	priv, err := readKey(tmpDir)
	require.Nil(t, err)

	const PubKeyAlgoEDDSA = 22
	require.Equal(t, fmt.Sprintf("%d", PubKeyAlgoEDDSA), fmt.Sprintf("%d", priv.PubKeyAlgo))
}

// Test that gpg stores well the keys in the "seed" format and reconstruct a
// "abstract.Scalar" from the dedis lib. and check if they yield the same
// public key. This is needed because Scalar automatically modulo the private
// key so we can't directly compare the gpg private key (non-modulo sha512) and
// the dedis one.
func TestGpgPrivateFormat(t *testing.T) {
	_, err := createKey(tmpDir, info)
	require.Nil(t, err)

	priv, err := readKey(tmpDir)
	require.Nil(t, err)
	golangXPriv, ok := priv.PrivateKey.(*ed25519.PrivateKey)
	require.True(t, ok)

	kyberPriv := scalarFromSeed(*golangXPriv)

	golangXPub := golangXPriv.Public()
	golangXPubBuff := golangXPub.(ed25519.PublicKey)

	kyberPub := suite.Point().Mul(nil, kyberPriv)
	kyberPubBuff, err := kyberPub.MarshalBinary()
	require.Nil(t, err)
	require.Equal(t, kyberPubBuff, []byte(golangXPubBuff))
}

// Test if the conversion from ed25519 to curve25519 is possible using gpg
// format. It convert the public key directly, then convert the secret key,
// do the base multiplication and then compare both results.
func TestEd25519ToCurve25519(t *testing.T) {
	_, err := createKey(tmpDir, info)
	require.Nil(t, err)

	priv, err := readKey(tmpDir)
	require.Nil(t, err)
	edwardsPrivate, ok := priv.PrivateKey.(*ed25519.PrivateKey)
	require.True(t, ok)

	kyberPrivate := scalarFromSeed(*edwardsPrivate)
	kyberPublic := suite.Point().Mul(nil, kyberPrivate)

	// convert edwards25519 private key to curve25519 private key
	var curve25519Private [32]byte
	var edwardsPrivateBuff [64]byte
	copy(edwardsPrivateBuff[:], *edwardsPrivate)
	extra25519.PrivateKeyToCurve25519(&curve25519Private, &edwardsPrivateBuff)
	// XXX extra try with kyber.Scalar
	/*b, err := kyberPrivate.MarshalBinary()*/
	//require.Nil(t, err)
	/*copy(curve25519Private[:], b)*/

	// convert edwards25519 public key to curve25519 public key
	var curve25519Public [32]byte
	var buffEdwardsPublic [32]byte
	buff, err := kyberPublic.MarshalBinary()
	require.Nil(t, err)
	copy(buffEdwardsPublic[:], buff)
	require.True(t, extra25519.PublicKeyToCurve25519(&curve25519Public, &buffEdwardsPublic))

	// Scalar multiplication of the curve25519 private key
	var curve25519Public2 [32]byte
	curve25519.ScalarBaseMult(&curve25519Public2, &curve25519Private)

	// check equality between the two public keys
	ret := bytes.Equal(curve25519Public2[:], curve25519Public[:])
	require.True(t, ret)
}

func TestNoisePreSharedKey(t *testing.T) {
	pub1, priv1, err := ed25519.GenerateKey(rand.Reader)
	require.Nil(t, err)
	pub2, priv2, err := ed25519.GenerateKey(rand.Reader)
	require.Nil(t, err)

	curvePub1, ok := ed25519PublicToCurve25519(&pub1)
	require.True(t, ok)
	curvePub2, ok := ed25519PublicToCurve25519(&pub2)
	require.True(t, ok)

	curvePriv1 := ed25519PrivateToCurve25519(&priv1)
	curvePriv2 := ed25519PrivateToCurve25519(&priv2)

	kp1 := noise.DHKey{
		Private: curvePriv1[:],
		Public:  curvePub1[:],
	}

	kp2 := noise.DHKey{
		Private: curvePriv2[:],
		Public:  curvePub2[:],
	}

	cs := noise.NewCipherSuite(noise.DH25519, noise.CipherAESGCM, noise.HashSHA256)
	hsI := noise.NewHandshakeState(noise.Config{
		CipherSuite:   cs,
		Pattern:       noise.HandshakeKK,
		Initiator:     true,
		StaticKeypair: kp1,
		PeerStatic:    kp2.Public,
	})
	hsR := noise.NewHandshakeState(noise.Config{
		CipherSuite:   cs,
		Pattern:       noise.HandshakeKK,
		StaticKeypair: kp2,
		PeerStatic:    kp1.Public,
	})

	// -> e,es,ss  msg
	msg, enc, dec := hsI.WriteMessage(nil, nil)
	require.Nil(t, enc)
	require.Nil(t, dec)
	res, enc, dec, err := hsR.ReadMessage(nil, msg)
	require.Nil(t, err)
	require.Nil(t, enc)
	require.Nil(t, dec)
	//c.Assert(err, IsNil)
	//c.Assert(string(res), Equals, "abc")

	// <- e, ee , se
	var msgNoise = []byte("abc")
	msg, cR0, cR1 := hsR.WriteMessage(nil, msgNoise)
	require.NotNil(t, cR0)
	require.NotNil(t, cR1)
	res, cI0, cI1, err := hsI.ReadMessage(nil, msg)
	require.Nil(t, err)
	require.NotEmpty(t, res)
	require.Equal(t, res, msgNoise)
	require.NotNil(t, cI0)
	require.NotNil(t, cI1)

	var clearMsg = []byte("Hello world")
	ciphertext := cI0.Encrypt(nil, nil, clearMsg)

	plain, err := cR0.Decrypt(nil, nil, ciphertext)
	require.Nil(t, err)
	require.Equal(t, clearMsg, plain)

	ciphertext = cR1.Encrypt(nil, nil, clearMsg)

	plain, err = cI1.Decrypt(nil, nil, ciphertext)
	require.Nil(t, err)
	require.Equal(t, clearMsg, plain)

}
