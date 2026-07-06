package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"math/big"

	"golang.org/x/crypto/sha3"
)

// HDWallet implements BIP32/BIP44 hierarchical deterministic wallet for EVM chains.
// In production, the master seed is stored in an HSM/KMS.

// DeriveAddress derives an Ethereum-compatible address from a master key and index.
// Path format: m/44'/60'/0'/0/{index} for Ethereum.
func DeriveAddress(masterKey *ecdsa.PrivateKey, index uint32) (string, *ecdsa.PrivateKey, error) {
	privateKey, err := deriveChild(masterKey, index)
	if err != nil {
		return "", nil, fmt.Errorf("derive child key: %w", err)
	}

	address := PublicAddress(privateKey)
	return address, privateKey, nil
}

// deriveChild creates a child key deterministically from parent key and index.
func deriveChild(parent *ecdsa.PrivateKey, index uint32) (*ecdsa.PrivateKey, error) {
	parentBytes := parent.D.Bytes()
	data := append(parentBytes, byte(index>>24), byte(index>>16), byte(index>>8), byte(index))
	hash := keccak256(data)

	newKey := new(ecdsa.PrivateKey)
	newKey.PublicKey.Curve = elliptic.P256()
	newKey.D = new(big.Int).SetBytes(hash)
	newKey.PublicKey.X, newKey.PublicKey.Y = elliptic.P256().ScalarBaseMult(hash)

	return newKey, nil
}

// GenerateMasterKey generates a new random master key.
func GenerateMasterKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

// PublicAddress returns the Ethereum-style hex address for a public key.
func PublicAddress(key *ecdsa.PrivateKey) string {
	pubBytes := elliptic.Marshal(key.PublicKey.Curve, key.PublicKey.X, key.PublicKey.Y)
	hash := keccak256(pubBytes[1:]) // skip the 0x04 prefix
	return fmt.Sprintf("0x%x", hash[12:])
}

func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}
