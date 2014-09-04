package mqtt

import (
	"bytes"
	"encoding/binary"
	"io"
	"fmt"
)

type PublishMessage struct {
	FixedHeader
	TopicName string
	PacketIdentifier uint16
	Payload []byte
	Opaque interface{}
}

func (self *PublishMessage) decode(reader io.Reader) error {
	var length uint16
	remaining := self.FixedHeader.RemainingLength
	binary.Read(reader, binary.BigEndian, &length)
	remaining -= 2

	buffer := bytes.NewBuffer(nil)
	_, err := io.CopyN(buffer, reader, int64(length))
	if err != nil {
		fmt.Printf("READ ERROR")
	}

	self.TopicName = string(buffer.Bytes())
	remaining -= int(length)
	if self.FixedHeader.QosLevel > 0 {
		binary.Read(reader, binary.BigEndian, &self.PacketIdentifier)
		remaining -= int(2)
	}

	buffer.Reset()
	_, err = io.CopyN(buffer, reader, int64(remaining))
	self.Payload = buffer.Bytes()

	return nil
}

func (self *PublishMessage) encode() ([]byte, int, error) {
	buffer := bytes.NewBuffer(nil)
	var size uint16 = uint16(len(self.TopicName))

	total := 2 + int(size)
	binary.Write(buffer, binary.BigEndian, size)
	buffer.Write([]byte(self.TopicName))

	if self.QosLevel > 0 {
		binary.Write(buffer, binary.BigEndian, self.PacketIdentifier)
		total += 2
	}
	buffer.Write(self.Payload)
	total += len(self.Payload)

	return buffer.Bytes(), total, nil
}
