package address

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/base64"
	"math/big"

	"github.com/calmw/tron-sdk/pkg/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	// HashLength is the expected length of the hash
	HashLength = 32
	// AddressLength is the expected length of the address
	AddressLength = 21
	// AddressLengthBase58 is the expected length of the address in base58format
	AddressLengthBase58 = 34
	// TronBytePrefix is the hex prefix to address
	TronBytePrefix = byte(0x41)
)

// Address represents the 21 byte address of an Tron account.
type Address []byte

// Bytes get bytes from address
func (a Address) Bytes() []byte {
	return a[:]
}

// Hex get bytes from address in string
func (a Address) Hex() string {
	return common.ToHex(a[:])
}

// BigToAddress returns Address with byte values of b.
// If b is larger than len(h), b will be cropped from the left.
func BigToAddress(b *big.Int) Address {
	id := b.Bytes()
	base := bytes.Repeat([]byte{0}, AddressLength-len(id))
	return append(base, id...)
}

// HexToAddress returns Address with byte values of s.
// If s is larger than len(h), s will be cropped from the left.
func HexToAddress(s string) Address {
	addr, err := common.FromHex(s)
	if err != nil {
		return nil
	}
	return addr
}

// Base58ToAddress returns Address with byte values of s.
func Base58ToAddress(s string) (Address, error) {
	addr, err := common.DecodeCheck(s)
	if err != nil {
		return nil, err
	}
	return addr, nil
}

// Base64ToAddress returns Address with byte values of s.
func Base64ToAddress(s string) (Address, error) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return Address(decoded), nil
}

// String implements fmt.Stringer.
func (a Address) String() string {
	if a[0] == 0 {
		return new(big.Int).SetBytes(a.Bytes()).String()
	}
	return common.EncodeCheck(a.Bytes())
}

// PubkeyToAddress returns address from ecdsa public key
func PubkeyToAddress(p ecdsa.PublicKey) Address {
	address := crypto.PubkeyToAddress(p)

	addressTron := make([]byte, 0)
	addressTron = append(addressTron, TronBytePrefix)
	addressTron = append(addressTron, address.Bytes()...)
	return addressTron
}
