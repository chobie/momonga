// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"bytes"
	"encoding/binary"
	"io"
)

type SubscribePayload struct {
	TopicPath string
	RequestedQos uint8
}

type SubscribeMessage struct {
	FixedHeader
	PacketIdentifier uint16
	Payload []SubscribePayload
}

func (self *SubscribeMessage) encode() ([]byte, int, error) {
	buffer := bytes.NewBuffer(nil)
	var total int = 0

	binary.Write(buffer, binary.BigEndian, self.PacketIdentifier)
	total += 2

	for i := 0; i < len(self.Payload); i++ {
		var length uint16 = uint16(len(self.Payload[i].TopicPath))
		binary.Write(buffer, binary.BigEndian, length)
		buffer.Write([]byte(self.Payload[i].TopicPath))
		binary.Write(buffer, binary.BigEndian, self.Payload[i].RequestedQos)

		total += 2 + len(self.Payload[i].TopicPath) + 1
	}

	return buffer.Bytes(), total, nil
}

func (self *SubscribeMessage) decode(reader io.Reader) error {
	remaining := self.RemainingLength

	binary.Read(reader, binary.BigEndian, &self.PacketIdentifier)
	remaining -= int(2)

	buffer := bytes.NewBuffer(nil)
	for remaining > 0 {
		var length uint16

		m := SubscribePayload{}
		binary.Read(reader, binary.BigEndian, &length)

		_, _ = io.CopyN(buffer, reader, int64(length))

		m.TopicPath = string(buffer.Bytes())
		binary.Read(reader, binary.BigEndian, &m.RequestedQos)
		self.Payload = append(self.Payload, m)

		buffer.Reset()
		remaining -= (int(length) + 1 + 2)
	}

	return nil
}
