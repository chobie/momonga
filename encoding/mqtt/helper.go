// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"encoding/binary"
	"errors"
	"io"
)

var overflow = errors.New("readvarint: varint overflows a 32-bit integer")

func ReadVarint(reader io.Reader) (int, error) {
	var RemainingLength uint8
	m := 1
	v := 0

	for i := 0; i < 4; i++ {
		binary.Read(reader, binary.BigEndian, &RemainingLength)
		v += (int(RemainingLength) & 0x7F) * m

		m *= 0x80
		if m > 0x200000 {
			return 0, overflow
		}
		if (RemainingLength & 0x80) == 0 {
			break
		}
	}

	return v, nil
}

func WriteVarint(writer io.Writer, size int) (int, error) {
	var encode_byte uint8
	x := size
	var i int

	for i = 0; x > 0 && i < 4; i++ {
		encode_byte = uint8(x % 0x80)
		x = x / 0x80
		if x > 0 {
			encode_byte |= 0x80
		}

		binary.Write(writer, binary.BigEndian, encode_byte)
	}

	return i, nil
}
