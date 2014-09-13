// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"bytes"
	"encoding/binary"
	"io"
	"encoding/json"
)

type ConnackMessage struct {
	FixedHeader
	Reserved uint8
	ReturnCode uint8
}

func (self *ConnackMessage) encode() ([]byte, int, error) {
	buffer := bytes.NewBuffer(nil)
	binary.Write(buffer, binary.BigEndian, self.Reserved)
	binary.Write(buffer, binary.BigEndian, self.ReturnCode)

	return buffer.Bytes(), 2, nil
}

func (self *ConnackMessage) decode(reader io.Reader) error {
	binary.Read(reader, binary.BigEndian, &self.Reserved)
	binary.Read(reader, binary.BigEndian, &self.ReturnCode)

	return nil
}

func (self ConnackMessage) WriteTo(w io.Writer) (int64, error) {
	var fsize = 2
	size, err := self.FixedHeader.writeTo(uint8(fsize), w)
	if err != nil {
		return 0, err
	}

	binary.Write(w, binary.BigEndian, self.Reserved)
	binary.Write(w, binary.BigEndian, self.ReturnCode)

	return int64(fsize)+size, nil
}

func (self *ConnackMessage) String() string {
	b, _ := json.Marshal(self)
	return string(b)
}

