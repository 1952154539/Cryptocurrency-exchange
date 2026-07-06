package wallet

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
	"golang.org/x/crypto/sha3"
)

// BIP32 constants
var (
	curve       = btcec.S256()
	curveOrder  = curve.Params().N
	hardenedKey = uint32(0x80000000)
)

// ExtendedKey holds a BIP32 extended key (private key + chain code + metadata).
type ExtendedKey struct {
	Key       *btcec.PrivateKey
	ChainCode []byte // 32 bytes
	Depth     uint8
	Index     uint32
}

// MasterKeyFromSeed derives a BIP32 master key from a seed (as produced by BIP39).
func MasterKeyFromSeed(seed []byte) (*ExtendedKey, error) {
	if len(seed) < 16 || len(seed) > 64 {
		return nil, fmt.Errorf("seed must be 16-64 bytes, got %d", len(seed))
	}

	mac := hmac.New(sha512.New, []byte("Bitcoin seed"))
	mac.Write(seed)
	I := mac.Sum(nil)

	IL, IR := I[:32], I[32:]

	keyNum := new(big.Int).SetBytes(IL)
	if keyNum.Cmp(curveOrder) >= 0 || keyNum.Sign() == 0 {
		return nil, fmt.Errorf("invalid master key: IL >= n or IL == 0")
	}

	privKey, _ := btcec.PrivKeyFromBytes(IL)

	return &ExtendedKey{
		Key:       privKey,
		ChainCode: IR,
		Depth:     0,
		Index:     0,
	}, nil
}

// DeriveChild derives a child extended key. Set index >= 0x80000000 for hardened derivation.
func (ek *ExtendedKey) DeriveChild(index uint32) (*ExtendedKey, error) {
	isHardened := index >= hardenedKey

	var data []byte
	if isHardened {
		data = append([]byte{0x00}, ek.Key.Serialize()...)
	} else {
		data = ek.Key.PubKey().SerializeCompressed()
	}
	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, index)
	data = append(data, indexBytes...)

	mac := hmac.New(sha512.New, ek.ChainCode)
	mac.Write(data)
	I := mac.Sum(nil)

	IL, IR := I[:32], I[32:]

	ILNum := new(big.Int).SetBytes(IL)
	if ILNum.Cmp(curveOrder) >= 0 {
		return nil, fmt.Errorf("invalid child key: IL >= n")
	}

	parentD := new(big.Int).SetBytes(ek.Key.Serialize())
	childD := new(big.Int).Add(ILNum, parentD)
	childD.Mod(childD, curveOrder)

	if childD.Sign() == 0 {
		return nil, fmt.Errorf("invalid child key: child == 0")
	}

	childKey, _ := btcec.PrivKeyFromBytes(childD.Bytes())

	return &ExtendedKey{
		Key:       childKey,
		ChainCode: IR,
		Depth:     ek.Depth + 1,
		Index:     index,
	}, nil
}

// PublicAddress returns the Ethereum-style hex address from a secp256k1 private key.
func PublicAddress(privKey *btcec.PrivateKey) string {
	pubBytes := privKey.PubKey().SerializeUncompressed()
	hash := keccak256(pubBytes[1:]) // skip 0x04 prefix
	return fmt.Sprintf("0x%x", hash[12:])
}

// DeriveBIP44Address derives an Ethereum address following BIP44 path: m/44'/60'/0'/0/{index}
func DeriveBIP44Address(masterKey *ExtendedKey, addressIndex uint32) (string, *btcec.PrivateKey, error) {
	return deriveBIP44WithAccount(masterKey, 0, addressIndex)
}

// DeriveBIP44AddressForUser derives a user-specific Ethereum address.
// Uses a per-user BIP44 account number to prevent address collisions across users.
// Path: m/44'/60'/{userAccount}'/0/{index}
func DeriveBIP44AddressForUser(masterKey *ExtendedKey, userID string, addressIndex uint32) (string, *btcec.PrivateKey, string, error) {
	// Hash userID to get a unique, deterministic account number per user
	userAccount := hashUserIDToAccount(userID)
	addr, key, err := deriveBIP44WithAccount(masterKey, userAccount, addressIndex)
	derivationPath := fmt.Sprintf("m/44'/60'/%d'/0/%d", hardenedKey+userAccount, addressIndex)
	return addr, key, derivationPath, err
}

func deriveBIP44WithAccount(masterKey *ExtendedKey, accountIndex, addressIndex uint32) (string, *btcec.PrivateKey, error) {
	purpose, err := masterKey.DeriveChild(hardenedKey + 44)
	if err != nil {
		return "", nil, fmt.Errorf("derive purpose: %w", err)
	}
	coinType, err := purpose.DeriveChild(hardenedKey + 60)
	if err != nil {
		return "", nil, fmt.Errorf("derive coin type: %w", err)
	}
	account, err := coinType.DeriveChild(hardenedKey + accountIndex)
	if err != nil {
		return "", nil, fmt.Errorf("derive account: %w", err)
	}
	change, err := account.DeriveChild(0)
	if err != nil {
		return "", nil, fmt.Errorf("derive change: %w", err)
	}
	child, err := change.DeriveChild(addressIndex)
	if err != nil {
		return "", nil, fmt.Errorf("derive address index: %w", err)
	}

	address := PublicAddress(child.Key)
	return address, child.Key, nil
}

// hashUserIDToAccount returns a BIP44 account number derived from the user ID.
// Uses keccak256 to produce a value in [0, 2^31 - 1] (non-hardened range).
func hashUserIDToAccount(userID string) uint32 {
	hash := keccak256([]byte(userID))
	return binary.BigEndian.Uint32(hash[:4]) & 0x7FFFFFFF
}

func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}
