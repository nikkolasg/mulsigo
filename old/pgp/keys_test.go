package pgp

import (
	"bytes"
	"crypto"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/ed25519"

	"github.com/agl/ed25519/extra25519"
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

/*func TestSplitEd25519Key(t *testing.T) {*/
//}

//func TestSignVerifyEd25519Key(t *testing.T) {
//var msg = []byte("Hello World")
//priv := createAndReadPrivateKey(t)

//var fname = path.Join(defaultTmpDir, "file")
//file, err := os.Create(fname)
//require.Nil(t, err)
//_, err = file.Write(msg)
//require.Nil(t, err)
//require.Nil(t, file.Close())

//reconstPriv := splitAndReconstruct(t, priv)
//sig := sign(t, reconstPriv, msg)
////sig := sign(t, priv, msg)

//var sigName = path.Join(defaultTmpDir, "testSig")
//f, err := os.Create(sigName)
//require.Nil(t, err)
//err = sig.Serialize(f)
//require.Nil(t, err)
//require.Nil(t, f.Close())

//// try to read with our lib
//buff, err := ioutil.ReadFile(sigName)
//require.Nil(t, err)
//var reader = bytes.NewBuffer(buff)
//unPack, err := packet.Read(reader)
//require.Nil(t, err)

//// try to read with gpg
//_, ok := unPack.(*packet.Signature)
//require.True(t, ok)

//verifyCmd := exec.Command("gpg", "--debug-level", "advanced", "--homedir", defaultTmpDir, "--verify", sigName, fname)
//out, err := verifyCmd.Output()
//fmt.Println("Output: ", string(out))
//if err != nil {
//log.Println(out)
//log.Println(err)
//t.Fail()
//}

//fmt.Println("OUTPUT: \n" + string(out))

//[>if !strings.Contains(strings.ToLower(string(out)), "good signature") {<]
////t.Fail()
//[>}<]

//}

//func TestCreateAndImport(t *testing.T) {
//CreateEd25519Key(batch)
//var b bytes.Buffer

//// read the public key
//readPublicArgs := []string{"--homedir", defaultTmpDir, "--export"}
//cmd := exec.Command("gpg", readPublicArgs...)
//cmd.Stdout = &b
//err := cmd.Run()
//require.Nil(t, err)

//buff := b.Bytes()
//reader := packet.NewReader(&b)
//p, err := reader.Next()
//require.Nil(t, err)
//fmt.Printf("reader.Next() #1: %+v, %s\n", p, err)

//pub, ok := p.(*packet.PublicKey)
//require.True(t, ok)

//p, err = reader.Next()
//fmt.Printf("reader.Next() #2: %+v, %s\n", p, err)
//p, err = reader.Next()
//fmt.Printf("reader.Next() #3: %+v, %s\n", p, err)
//p, err = reader.Next()
//fmt.Printf("reader.Next() #4: %+v, %s\n", p, err)

//// serialize and import it in another dir
//var gpgSerialized bytes.Buffer
//require.Nil(t, pub.Serialize(&gpgSerialized))

//var test2 = "test2"
//os.Mkdir(test2, 0764)
//require.Nil(t, ioutil.WriteFile("test2.pub", gpgSerialized.Bytes(), 0744))
////defer os.Remove("test2.pub")
//defer os.RemoveAll(test2)
//cmd = exec.Command("gpg", "--homedir", test2, "--import", "test2.pub")
//out, err := cmd.CombinedOutput()
//fmt.Println("Import: ", string(out))
//if err != nil {
//fmt.Println("Import err: ", err.Error())
//}
//require.Nil(t, err)

//fmt.Printf("GPG Pub %+v\n", pub)

//constructed := packet.NewEDDSAPublicKey(pub.CreationTime, pub.PublicKey.(*ed25519.PublicKey))
//var serialized bytes.Buffer
//constructed.Serialize(&serialized)

//enc := hex.EncodeToString
//fmt.Printf("Golang Pub %+v\n", constructed)
//require.Equal(t, enc(serialized.Bytes()), enc(buff))

//}

//func TestCreateEntity(t *testing.T) {
//var name = "chewbakka"
//var email = "chewbakka@millenium.com"
//var comment = "big hairy beast"
//ent, err := openpgp.NewEDDSAEntity(name, comment, email, nil)
//require.Nil(t, err)

//var fname = "testEntity.gpg"
//var gpgDir = "testEntity"
//f, err := os.Create(fname)
//require.Nil(t, err)
//os.Mkdir(gpgDir, 0766)

//defer os.Remove(fname)
//defer os.RemoveAll(gpgDir)

//require.Nil(t, ent.Serialize(f))
//require.Nil(t, f.Close())

//cmd := exec.Command("gpg", "--homedir", gpgDir, "-vvv", "--import", fname)
//out, err := cmd.CombinedOutput()
//fmt.Println("IMPORT: ", string(out))
//require.Nil(t, err)
//}

//[>func clonePacketPrivate(t *testing.T, priv *packet.PrivateKey) *packet.PrivateKey {<]
////var privc ed25519.PrivateKey
////var pr = priv.PrivateKey.(*ed25519.PrivateKey)
////copy(privc[:], (*pr)[:])
//[>}<]

//func splitAndReconstruct(t *testing.T, priv *packet.PrivateKey) *packet.PrivateKey {
//var ppriv = priv.PrivateKey.(*ed25519.PrivateKey)
//ed := sha512.Sum512((*ppriv)[:32])

//ed[0] &= 248
//ed[31] &= 127
//ed[31] |= 64

//var k = 4
//var n = 6
//var secret PrivateKey
//copy(secret[:], (ed)[:])

//scalar := secret.Scalar()
//public := scalar.Commit()

//poly, err := NewPoly(rand.Reader, scalar, public, uint32(k))
//require.Nil(t, err)

//shares := make([]Share, n)
//for i := 0; i < int(n); i++ {
//shares[i] = poly.Share(uint32(i))
//}

//recons, err := Reconstruct(shares, uint32(k), uint32(n))
//assert.Nil(t, err)
//assert.True(t, scalar.Equal(recons.Int))
//reconsPub := recons.Commit()

//ed25519Secret := append(recons.Bytes(), reconsPub.Bytes()...)
//pointer := ed25519.PrivateKey(ed25519Secret)

////creationTime := fetchCreationTime()
//// XXX Creation time should be taken out of the public key
//reconsPacket := packet.NewEDDSAPrivateKey(priv.CreationTime, &pointer)

//var b1 bytes.Buffer
//var b2 bytes.Buffer
//reconsPacket.PublicKey.Serialize(&b1)
//priv.PublicKey.Serialize(&b2)

//describe(reconsPacket, priv)

//require.Equal(t, hex.EncodeToString(b1.Bytes()), hex.EncodeToString(b2.Bytes()))
//return reconsPacket
//}

//func fetchKey(email, homedir string) {
//[>buff, err := exec.Command("gpg", "--homedir", homedir, "--export", email).Output()<]
////p, err := packet.Read(bytes.NewBuffer(buff))
////public := p.(*packet.PublicKey)

//}

//func describe(p1, p2 *packet.PrivateKey) {
//fmt.Printf("Private Creation Time #1: %v\n", p1.CreationTime)
//fmt.Printf("Private Creation Time #2: %v\n", p2.CreationTime)
//pk1 := p1.PrivateKey.(*ed25519.PrivateKey)
//pk2 := p2.PrivateKey.(*ed25519.PrivateKey)
//fmt.Printf("Private Key #1: %v \n", hex.EncodeToString(*pk1))
//fmt.Printf("Private Key #1: %v \n", hex.EncodeToString(*pk2))
//pb1 := p1.PublicKey
//pb2 := p2.PublicKey
//fmt.Printf("Public Creation Time #1: %v\n", pb1.CreationTime)
//fmt.Printf("Public Creation Time #2: %v\n", pb2.CreationTime)
//epb1 := pb1.PublicKey.(*ed25519.PublicKey)
//epb2 := pb2.PublicKey.(*ed25519.PublicKey)
//fmt.Printf("Public Key #1: %v \n", hex.EncodeToString(*epb1))
//fmt.Printf("Public Key #2: %v \n", hex.EncodeToString(*epb2))
//}

//func createAndReadPrivateKey(t *testing.T) *packet.PrivateKey {
//CreateEd25519Key(batch)
//p, err := ReadEd25519Key()
//require.Nil(t, err)
//return p
//}

//func Scheme(rand io.Reader, privateI interface{}, msg []byte) ([]byte, error) {
//pr, ok := privateI.(ed25519.PrivateKey)
//if !ok {
//return nil, errors.New("private key not being *ed25519.PrivateKey" + reflect.TypeOf(privateI).String())
//}
//var private PrivateKey
//copy(private[:], pr[:])
//buff := SchnorrSign(private.Scalar(), msg, rand)
//return buff[:], nil
//}

//func sign(t *testing.T, priv *packet.PrivateKey, msg []byte) *packet.Signature {
//if priv.PubKeyAlgo != PubKeyAlgoEDDSA {
//t.Fatal("NewSignerPrivateKey should have made a ECSDA private key")
//}

//sig := &packet.Signature{
//PubKeyAlgo:  PubKeyAlgoEDDSA,
//Hash:        crypto.SHA256,
//SigType:     packet.SigTypeBinary,
//IssuerKeyId: &priv.KeyId,
//}

//h := crypto.SHA256.New()
//_, err := h.Write(msg)
//require.Nil(t, err)

//err = sig.SignWithScheme(h, priv, nil, Scheme)
//require.Nil(t, err)

//h = crypto.SHA256.New()
//_, err = h.Write(msg)
//require.Nil(t, err)

//err = priv.VerifySignature(h, sig)
//require.Nil(t, err)

//return sig
//}

//func readFakePrivateKey(fake string) *packet.PrivateKey {
//b, _ := hex.DecodeString(fake)
//var buff = bytes.NewBuffer(b)
//p, _ := packet.Read(buff)

//priv, _ := p.(*packet.PrivateKey)
//return priv
/*}*/
