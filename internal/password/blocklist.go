package password

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"strings"
)

const bloomHeaderSize = 7 + 8 + 4

//go:generate go run ./cmd/generate-blocklist

// common-passwords.bloom is generated from the pinned SecLists top-100,000
// common credential list. See BLOCKLIST.md for provenance and reproduction.
//
//go:embed common-passwords.bloom
var commonPasswordBloom []byte

type bloomBlocklist struct {
	bits []byte
	size uint64
	keys uint32
}

func BuiltinBlocklist() Blocklist {
	if len(commonPasswordBloom) < bloomHeaderSize ||
		string(commonPasswordBloom[:7]) != "YLPWBL1" {
		panic("password blocklist asset is invalid")
	}
	size := binary.BigEndian.Uint64(commonPasswordBloom[7:15])
	keys := binary.BigEndian.Uint32(commonPasswordBloom[15:19])
	bits := commonPasswordBloom[19:]
	if size == 0 || keys == 0 || uint64(len(bits))*8 < size {
		panic("password blocklist asset has invalid dimensions")
	}
	return bloomBlocklist{bits: bits, size: size, keys: keys}
}

func (blocklist bloomBlocklist) Contains(
	_ context.Context,
	value string,
) (bool, error) {
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(value))))
	first := binary.BigEndian.Uint64(digest[:8])
	second := binary.BigEndian.Uint64(digest[8:16])
	if second == 0 {
		second = 0x9e3779b97f4a7c15
	}
	for index := uint32(0); index < blocklist.keys; index++ {
		position := (first + uint64(index)*second) % blocklist.size
		if blocklist.bits[position/8]&(1<<(position%8)) == 0 {
			return false, nil
		}
	}
	return true, nil
}
