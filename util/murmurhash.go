// this function comes from
// https://github.com/stf-storage/go-stf-server/blob/master/murmurhash.go
//
// Author: github.com/lestrrat

package util

import (
	"bytes"
	"encoding/binary"
)

func MurmurHash(data []byte) uint32 {
	const m uint32 = 0x5bd1e995
	const r uint8 = 16
	var length uint32 = uint32(len(data))
	var h uint32 = length * m

	nblocks := int(length / 4)
	buf := bytes.NewBuffer(data)
	for i := 0; i < nblocks; i++ {
		var x uint32
		err := binary.Read(buf, binary.LittleEndian, &x)
		if err != nil {
			return 0
		}
		h += x
		h *= m
		h ^= h >> r
	}

	tailIndex := nblocks * 4
	switch length & 3 {
	case 3:
		h += uint32(data[tailIndex+2]) << 16
		fallthrough
	case 2:
		h += uint32(data[tailIndex+1]) << 8
		fallthrough
	case 1:
		h += uint32(data[tailIndex])
		h *= m
		h ^= h >> r
	}

	h *= m
	h ^= h >> 10
	h *= m
	h ^= h >> 17

	return h
}
