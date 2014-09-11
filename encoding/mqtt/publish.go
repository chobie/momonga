// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

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

func (self *PublishMessage) GetPacketIdentifierBytes() (identifier []byte) {
	if self.QosLevel > 0 {
		ww := bytes.NewBuffer(nil)
		binary.Write(ww, binary.BigEndian, self.PacketIdentifier)
		identifier = ww.Bytes()
	}

	return
}

func (self *PublishMessage) Encode2() (topic []byte, identifier []byte, payload []byte, total int) {
	// fixed header
	//(2+n)topic
	//[(2)packet_identifier]
	//(n)payload
	//(int) size

	var size uint16 = uint16(len(self.TopicName))
	total = 2 + int(size)
	if self.QosLevel > 0 {
		total += 2
	}
	total += len(self.Payload)

	buffer := bytes.NewBuffer(nil)
	binary.Write(buffer, binary.BigEndian, size)
	buffer.Write([]byte(self.TopicName))
	topic = buffer.Bytes()

	if self.QosLevel > 0 {
		ww := bytes.NewBuffer(nil)
		binary.Write(ww, binary.BigEndian, self.PacketIdentifier)
		identifier = ww.Bytes()
	}

	payload = self.Payload
	return

}
