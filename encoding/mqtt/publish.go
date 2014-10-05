// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

type PublishMessage struct {
	FixedHeader      `json:"header"`
	TopicName        string      `json:"topic_name"`
	PacketIdentifier uint16      `json:"identifier"`
	Payload          []byte      `json:"payload"`
	Opaque           interface{} `json:"-"`
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
		if offset > remaining {
			panic("something went to wrong(offset overs remianing length)")
		}

		i, err := reader.Read(buffer[offset:])
		offset += i
		if err != nil && offset < remaining {
			// if we read whole size of message, ignore error at this time.
			return err
		}
	}

	if int(length) > len(buffer) {
		return fmt.Errorf("publish length: %d, buffer: %d", length, len(buffer))
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

	header_len, e := self.FixedHeader.writeTo(total, w)
	if e != nil {
		return 0, e
	}
	total += int(size)

	e = binary.Write(w, binary.BigEndian, size)
	if e != nil {
		return 0, e
	}
	w.Write([]byte(self.TopicName))
	if self.QosLevel > 0 {
		e = binary.Write(w, binary.BigEndian, self.PacketIdentifier)
	}
	if e != nil {
		return 0, e
	}
	_, e = w.Write(self.Payload)
	if e != nil {
		return 0, e
	}

	return int64(int(total) + int(header_len)), nil
}

func (self *PublishMessage) String() string {
	b, _ := json.Marshal(self)
	return string(b)
}
