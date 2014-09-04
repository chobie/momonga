package mqtt

import (
	"bytes"
	"encoding/binary"
	"io"
)

type PubcompMessage struct {
	FixedHeader
	PacketIdentifier uint16
}

func (self *PubcompMessage) encode() ([]byte, int, error) {
	buffer := bytes.NewBuffer(nil)
	binary.Write(buffer, binary.BigEndian, self.PacketIdentifier)
	return buffer.Bytes(), 2, nil
}

func (self *PubcompMessage) decode(reader io.Reader) error {
	binary.Read(reader, binary.BigEndian, &self.PacketIdentifier)
	return nil
}
