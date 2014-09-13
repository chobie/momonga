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

type UnsubackMessage struct {
	FixedHeader
	PacketIdentifier uint16
}

func (self *UnsubackMessage) encode() ([]byte, int, error) {
	buffer := bytes.NewBuffer(nil)
	binary.Write(buffer, binary.BigEndian, self.PacketIdentifier)
	return buffer.Bytes(), 2, nil
}

func (self *UnsubackMessage) WriteTo(w io.Writer) (int64, error) {
	var fsize = 2
	size, err := self.FixedHeader.writeTo(uint8(fsize), w)
	if err != nil {
		return 0, err
	}

	binary.Write(w, binary.BigEndian, self.PacketIdentifier)
	return int64(size) + int64(fsize), nil
}

func (self *UnsubackMessage) decode(reader io.Reader) error {
	binary.Read(reader, binary.BigEndian, &self.PacketIdentifier)
	return nil
}

func (self *UnsubackMessage) String() string {
	b, _ := json.Marshal(self)
	return string(b)
}
