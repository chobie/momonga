package mqtt

import (
	"bytes"
	"encoding/binary"
	"io"
)

type SubackMessage struct {
	FixedHeader
	PacketIdentifier uint16
	Qos []byte
}

func (self *SubackMessage) encode() ([]byte, int, error) {
	buffer := bytes.NewBuffer(nil)
	var total int = 0
	var QoS uint8 = 0

	binary.Write(buffer, binary.BigEndian, self.PacketIdentifier)
	total += 2
	// TODO: 実装ちゃんとやる
	binary.Write(buffer, binary.BigEndian, QoS)
	total += 1

	return buffer.Bytes(), total, nil
}

func (self *SubackMessage) decode(reader io.Reader) error {
	var remaining uint8
	remaining = uint8(self.FixedHeader.RemainingLength)
	binary.Read(reader, binary.BigEndian, &self.PacketIdentifier)

	remaining -= 2
	buffer := bytes.NewBuffer(nil)
	for i := 0; i <= int(remaining); i++ {
		var value uint8 = 0
		binary.Read(reader, binary.BigEndian, &value)
		binary.Write(buffer, binary.BigEndian, value)
		remaining -= 1
	}

	self.Qos = buffer.Bytes()
	return nil
}
