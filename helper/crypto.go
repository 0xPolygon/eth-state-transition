package helper

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"

	"github.com/0xPolygon/eth-state-transition/types"
	"github.com/btcsuite/btcd/btcec"
)

// S256 is the secp256k1 elliptic curve
var S256 = btcec.S256()

var (
	secp256k1N = MustDecodeHex("0xfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141")
	one        = []byte{0x01}
)

func trimLeftZeros(b []byte) []byte {
	i := 0
	for i = range b {
		if b[i] != 0 {
			break
		}
	}
	return b[i:]
}

// ValidateSignatureValues checks if the signature values are correct
func ValidateSignatureValues(v byte, r, s []byte) bool {
	// TODO: ECDSA malleability
	if v > 1 {
		return false
	}

	r = trimLeftZeros(r)
	if bytes.Compare(r, secp256k1N) >= 0 || bytes.Compare(r, one) < 0 {
		return false
	}

	s = trimLeftZeros(s)
	if bytes.Compare(s, secp256k1N) >= 0 || bytes.Compare(s, one) < 0 {
		return false
	}
	return true
}

// MarshalPublicKey marshals a public key on the secp256k1 elliptic curve.
func MarshalPublicKey(pub *ecdsa.PublicKey) []byte {
	return elliptic.Marshal(S256, pub.X, pub.Y)
}

func Ecrecover(hash, sig []byte) ([]byte, error) {
	pub, err := RecoverPubkey(sig, hash)
	if err != nil {
		return nil, err
	}
	return MarshalPublicKey(pub), nil
}

// RecoverPubkey verifies the compact signature "signature" of "hash" for the
// secp256k1 curve.
func RecoverPubkey(signature, hash []byte) (*ecdsa.PublicKey, error) {
	size := len(signature)
	term := byte(27)
	if signature[size-1] == 1 {
		term = 28
	}

	sig := append([]byte{term}, signature[:size-1]...)
	pub, _, err := btcec.RecoverCompact(S256, sig, hash)
	if err != nil {
		return nil, err
	}
	return pub.ToECDSA(), nil
}

func ParsePrivateKey(buf []byte) (*ecdsa.PrivateKey, error) {
	prv, _ := btcec.PrivKeyFromBytes(S256, buf)
	return prv.ToECDSA(), nil
}

// PubKeyToAddress returns the Ethereum address of a public key
func PubKeyToAddress(pub *ecdsa.PublicKey) types.Address {
	buf := Keccak256(MarshalPublicKey(pub)[1:])[12:]
	return types.BytesToAddress(buf)
}
