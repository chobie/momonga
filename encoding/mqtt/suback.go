// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
)

type SubackMessage struct {
	FixedHeader
	PacketIdentifier uint16
	Qos              []byte
}

func (self *SubackMessage) encode() ([]byte, int, error) {
	buffer := bytes.NewBuffer(nil)
	var total int = 0

	binary.Write(buffer, binary.BigEndian, self.PacketIdentifier)
	total += 2

	io.Copy(buffer, bytes.NewReader(self.Qos))
	total += len(self.Qos)

	return buffer.Bytes(), total, nil
}

func (self *SubackMessage) WriteTo(w io.Writer) (int64, error) {
	var fsize = 2 + len(self.Qos)

	size, err := self.FixedHeader.writeTo(uint8(fsize), w)
	if err != nil {
		return 0, err
	}

	binary.Write(w, binary.BigEndian, self.PacketIdentifier)
	io.Copy(w, bytes.NewReader(self.Qos))

	return int64(size) + int64(fsize), nil
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

func (self *SubackMessage) String() string {
	b, _ := json.Marshal(self)
	return string(b)
}
