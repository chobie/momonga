package mqtt

import (
	"bytes"
	"encoding/binary"
	"io"
)

type UnsubscribeMessage struct {
	FixedHeader
	TopicName string
	Identifier uint16
	Payload []SubscribePayload
}

func (self *UnsubscribeMessage) decode(reader io.Reader) error {
	remaining := self.RemainingLength

	binary.Read(reader, binary.BigEndian, &self.Identifier)
	remaining -= int(2)

	buffer := bytes.NewBuffer(nil)
	for remaining > 0 {
		var length uint16 = 0

		m := SubscribePayload{}
		binary.Read(reader, binary.BigEndian, &length)
		_, _ = io.CopyN(buffer, reader, int64(length))
		m.TopicFilter = string(buffer.Bytes())
		buffer.Reset()
		self.Payload = append(self.Payload, m)
		remaining -= (int(length) + 1 + 2)
	}

	return nil
}

func (self *UnsubscribeMessage) encode() ([]byte, int, error) {
	buffer := bytes.NewBuffer(nil)
	var total = 0

	binary.Write(buffer, binary.BigEndian, self.Identifier)
	total += 2

	for i := 0; i < len(self.Payload); i++ {
		var length uint16 = 0
		length = uint16(len(self.Payload[i].TopicFilter))
		binary.Write(buffer, binary.BigEndian, length)
		buffer.Write([]byte(self.Payload[i].TopicFilter))
		total += 2 + int(length)
	}

	return buffer.Bytes(), total, nil
}
