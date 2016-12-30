// The file provides the primitives to create a valid pgp signature verifiable
// by the EDDSA module.
// Most of the code is either directly taken or adapted from:
// https://github.com/sriak/cothority/
package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"hash"
	"io"
	"time"
)

const PubKeyAlgoEDDSA = 22
const maxOIDLength = 9
const packetTypeUserID = 13
const packetTypePublicKey = 6
const packetTypeSignature = 2

// Taken from https://tools.ietf.org/html/draft-ietf-openpgp-rfc4880bis-00#section-9.2
var oid = []byte{0x2B, 0x06, 0x01, 0x04, 0x01, 0xDA, 0x47, 0x0F, 0x01}

// Creates the hash of the message to be signed cf. 4880 section 5.2.4
func HashMessage(h hash.Hash, msg []byte) []byte {
	h.Write(msg)
	var buf []byte
	// signature version 4
	buf = append(buf, byte(4))
	// data type binary (0)
	buf = append(buf, byte(0))
	// pub key algo
	buf = append(buf, byte(PubKeyAlgoEDDSA))
	// hash algo sha256
	buf = append(buf, byte(8))

	// scalar octect count for hashed subpacket data
	hashSubacketLength := 0
	buf = append(buf, byte(hashSubacketLength>>8))
	buf = append(buf, byte(hashSubacketLength))

	// trailer
	buf = append(buf, 0x04)
	buf = append(buf, 0xff)
	// total length = header + subpacket length
	l := 6 + hashSubacketLength
	buf = append(buf, byte(l>>24))
	buf = append(buf, byte(l>>16))
	buf = append(buf, byte(l>>8))
	buf = append(buf, byte(l))
	h.Write(buf)
	return h.Sum(nil)
}

// Serializes into the given writer the signature from the pubkey, the input
// data and the point R and the integer S cf. RFC 4880 section 5.2
func SerializeSignature(w io.Writer, data, pubKey, r, s []byte) (err error) {
	// We prepend the pubKey with 0x40 to indicate that it is compressed cf.
	// https://tools.ietf.org/html/draft-ietf-openpgp-rfc4880bis-00#section-13.3
	pubKey = append([]byte{0x40}, pubKey...)

	// Get the key id cf. https://tools.ietf.org/html/rfc4880#section-12.2
	keyID := keyID(pubKey)

	// Header length
	length := 2

	dataSig := signaturePacket(data, r, s, keyID)
	length += len(dataSig)

	err = serializeHeader(w, packetTypeSignature, length)
	if err != nil {
		return
	}
	w.Write(dataSig)
	return
}

// Serializes into the given writer the public key and the user id cf. RFC 4880
// section 5.5.1.1
func SerializePubKey(w io.Writer, pubKey []byte, userID string) (err error) {
	// We prepend the pubKey with 0x40 to indicate that it is compressed cf.
	// https://tools.ietf.org/html/draft-ietf-openpgp-rfc4880bis-00#section-13.3
	pubKey = append([]byte{0x40}, pubKey...)
	// Packet header size + packet size (1 octet of size since size < 192)
	length := 2
	// Version number = 4
	length += 1
	// Creation time
	length += 4
	// public key algo
	length += 1
	// size of OID
	length += 1
	// OID length
	length += len(oid)
	// MPI Point length
	length += len(pubKey)
	packetType := packetTypePublicKey
	err = serializeHeader(w, packetType, length)
	if err != nil {
		return
	}
	err = serializePubKeyWithoutHeader(w, pubKey)
	if err != nil {
		return
	}
	err = serializeUserID(w, userID)
	return

}

// Writes an user id. cf. RFC 4880 section 5.11
func serializeUserID(w io.Writer, userId string) (err error) {
	bytesId := []byte(userId)
	// Packet header + size
	length := 2
	// userId lendth
	length += len(bytesId)
	serializeHeader(w, packetTypeUserID, length)
	_, err = w.Write(bytesId)
	return
}

// Writes an openpgp packet header cf. RFC 4880 section 4.2
func serializeHeader(w io.Writer, ptype int, length int) (err error) {
	var buf [6]byte
	var n int

	buf[0] = 0x80 | 0x40 | byte(ptype)
	if length < 192 {
		buf[1] = byte(length)
		n = 2
	} else if length < 8384 {
		length -= 192
		buf[1] = 192 + byte(length>>8)
		buf[2] = byte(length)
		n = 3
	} else {
		buf[1] = 255
		buf[2] = byte(length >> 24)
		buf[3] = byte(length >> 16)
		buf[4] = byte(length >> 8)
		buf[5] = byte(length)
		n = 6
	}

	_, err = w.Write(buf[:n])
	return
}

func serializePubKeyWithoutHeader(w io.Writer, pubKey []byte) (err error) {
	var buf []byte
	// Version number 4
	buf = append(buf, byte(4))

	t := uint32(time.Now().Unix())
	buf = append(buf, byte(t>>24))
	buf = append(buf, byte(t>>16))
	buf = append(buf, byte(t>>8))
	buf = append(buf, byte(t))

	// Algo used for public key is EDDSA
	buf = append(buf, byte(PubKeyAlgoEDDSA))

	// OID of the curve used, here ed25519 cf. https://tools.ietf.org/html/draft-ietf-openpgp-rfc4880bis-00#section-9.2
	buf = append(buf, byte(len(oid)))
	buf = append(buf, oid...)

	// Public key size is actually 263 bits cf https://tools.ietf.org/html/draft-ietf-openpgp-rfc4880bis-00#section-13.3
	bitLength := uint16(8*len(pubKey) - 1)
	buf = append(buf, byte(bitLength>>8))
	buf = append(buf, byte(bitLength))

	buf = append(buf, pubKey...)

	_, err = w.Write(buf)

	return
}

func signaturePacket(data, r, s, keyID []byte) (sig []byte) {
	var buf []byte
	// Version 4
	buf = append(buf, byte(4))
	// data type binary (0)
	buf = append(buf, byte(0))
	// pub key algo
	buf = append(buf, byte(PubKeyAlgoEDDSA))
	// hash algo sha256
	buf = append(buf, byte(8))

	// scalar octect count for hashed subpacket data
	hashSubpacketLength := 0
	buf = append(buf, byte(hashSubpacketLength>>8))
	buf = append(buf, byte(hashSubpacketLength))

	hasher := sha256.New()

	signedHashValue := HashMessage(hasher, data)

	// scalar octet count for unashed subpacket data
	buf = append(buf, byte(0))
	buf = append(buf, byte(10))
	// subpacket length
	buf = append(buf, byte(9))
	// subpacket type 16 = issuer key ID
	buf = append(buf, byte(16))
	// 8 octet issuer key ID
	buf = append(buf, keyID...)

	buf = append(buf, signedHashValue[:2]...)
	// Length of MPI in bits 256
	length := uint16(8 * len(r))
	buf = append(buf, byte(length>>8))
	buf = append(buf, byte(length))
	buf = append(buf, r...)
	// Length of MPI in bits 256
	length = uint16(8 * len(s))
	buf = append(buf, byte(length>>8))
	buf = append(buf, byte(length))
	buf = append(buf, s...)
	return buf
}

// Gets the ID of the given public key cf. RFC 4880 section 12.2
func keyID(pubKey []byte) (id []byte) {
	serializeBuf := bytes.NewBuffer(nil)
	serializePubKeyWithoutHeader(serializeBuf, pubKey)
	length := len(serializeBuf.Bytes())
	fingerPrint := sha1.New()
	fingerPrint.Write([]byte{0x99, byte(length >> 8), byte(length)})
	serializePubKeyWithoutHeader(fingerPrint, pubKey)
	return fingerPrint.Sum(nil)[12:20]
}
