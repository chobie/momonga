package mqtt

import (
	"encoding/binary"
	"io"
)

func ReadVarint(reader io.Reader) (int, error){
	var RemainingLength uint8
	m := 1
	v := 0
	for i := 0; i < 4; i++ {
		binary.Read(reader, binary.BigEndian, &RemainingLength)
		v += (int(RemainingLength) & 0x7F) * m

		m *= 0x80
		if m > 0x200000 {
			return 0, nil
		}
		if (RemainingLength & 128) == 0 {
			break
		}
	}

	return v, nil
}
