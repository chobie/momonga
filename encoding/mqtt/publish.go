// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"bytes"
	"encoding/binary"
	"io"
	 "fmt"
	"encoding/json"
)

type PublishMessage struct {
	FixedHeader `json:"header"`
	TopicName string `json:"topic_name"`
	PacketIdentifier uint16 `json:"identifier"`
	Payload []byte `json:"payload"`
	Opaque interface{} `json:"-"`
}

func (self *PublishMessage) decode(reader io.Reader) error {
	var length uint16
	remaining := self.FixedHeader.RemainingLength
	binary.Read(reader, binary.BigEndian, &length)
	remaining -= 2

	if remaining < 1 {
		return fmt.Errorf("something wrong. (probably crouppted data?)")
	}

	buffer := make([]byte, remaining)
	offset := 0
	for offset < remaining {
		i, err := reader.Read(buffer[offset:])
		if err != nil {
			return fmt.Errorf("PublishMessage::Decode: %s", err)
		}
		offset += i
	}

	self.TopicName = string(buffer[0:length])
	payload_offset := length
	if self.FixedHeader.QosLevel > 0 {
		binary.Read(bytes.NewReader(buffer[length:]), binary.BigEndian, &self.PacketIdentifier)
		payload_offset += 2
	}
	self.Payload = buffer[payload_offset:]

	return nil
}

func (self *PublishMessage) WriteTo(w io.Writer) (int64, error) {
	var size uint16 = uint16(len(self.TopicName))
	total := 2 + int(size)
	if self.QosLevel > 0 {
		total += 2
	}
	total += len(self.Payload)

	header_len, _ := self.FixedHeader.writeTo(uint8(total), w)
	total += int(size)

	binary.Write(w, binary.BigEndian, size)
	w.Write([]byte(self.TopicName))
	if self.QosLevel > 0 {
		binary.Write(w, binary.BigEndian, self.PacketIdentifier)
	}
	w.Write(self.Payload)

	return int64(int(total) + int(header_len)), nil
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

func (self *PublishMessage) String() string {
	b, _ := json.Marshal(self)
	return string(b)
}
