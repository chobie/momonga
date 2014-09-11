// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"bytes"
	"encoding/binary"
	"io"
)

type UnsubscribeMessage struct {
	FixedHeader
	TopicName string
	PacketIdentifier uint16
	Payload []SubscribePayload
}

func (self *UnsubscribeMessage) decode(reader io.Reader) error {
	remaining := self.RemainingLength

	binary.Read(reader, binary.BigEndian, &self.PacketIdentifier)
	remaining -= int(2)

	buffer := bytes.NewBuffer(nil)
	for remaining > 0 {
		var length uint16 = 0

		m := SubscribePayload{}
		binary.Read(reader, binary.BigEndian, &length)
		_, _ = io.CopyN(buffer, reader, int64(length))
		m.TopicPath = string(buffer.Bytes())
		buffer.Reset()
		self.Payload = append(self.Payload, m)
		remaining -= (int(length) + 1 + 2)
	}

	return nil
}

func (self *UnsubscribeMessage) encode() ([]byte, int, error) {
	buffer := bytes.NewBuffer(nil)
	var total = 0

	binary.Write(buffer, binary.BigEndian, self.PacketIdentifier)
	total += 2

	for i := 0; i < len(self.Payload); i++ {
		var length uint16 = 0
		length = uint16(len(self.Payload[i].TopicPath))
		binary.Write(buffer, binary.BigEndian, length)
		buffer.Write([]byte(self.Payload[i].TopicPath))
		total += 2 + int(length)
	}

	return buffer.Bytes(), total, nil
}
