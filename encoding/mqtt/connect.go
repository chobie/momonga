// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	//	"encoding/hex"
	"encoding/json"
)

type ConnectMessage struct {
	FixedHeader
	Magic        []byte       `json:"magic"`
	Version      uint8        `json:"version"`
	Flag         uint8        `json:"flag"`
	KeepAlive    uint16       `json:"keep_alive"`
	Identifier   string       `json:"identifier"`
	Will         *WillMessage `json:"will"`
	CleanSession bool         `json:clean_session`
	UserName     string       `json:"user_name"`
	Password     string       `json:"password"`
}

func (self *ConnectMessage) WriteTo(w io.Writer) (int64, error) {
	var headerLength uint16 = uint16(len(self.Magic))
	var size int = 0

	if self.CleanSession {
		self.Flag |= 0x02
	}
	if self.Will != nil {
		self.Flag |= 0x04
		switch self.Will.Qos {
		case 1:
			self.Flag |= 0x08
		case 2:
			self.Flag |= 0x18
		}
	}
	if len(self.UserName) > 0 {
		self.Flag |= 0x80
	}
	if len(self.Password) > 0 {
		self.Flag |= 0x40
	}

	size += 2 + len(self.Magic)
	size += 1 + 1 + 2
	if self.Identifier != "" {
		size += 2 + len(self.Identifier)
	}
	if (int(self.Flag)&0x04 > 0) && self.Will != nil {
		size += self.Will.Size()
	}
	if int(self.Flag)&0x80 > 0 {
		size += 2 + len(self.UserName)
	}
	if int(self.Flag)&0x40 > 0 {
		size += 2 + len(self.Password)
	}

	self.FixedHeader.writeTo(uint8(size), w)
	err := binary.Write(w, binary.BigEndian, headerLength)
	if err != nil {
		fmt.Printf("1Error: %s\n", err)
	}

	w.Write(self.Magic)
	binary.Write(w, binary.BigEndian, self.Version)
	binary.Write(w, binary.BigEndian, self.Flag)
	binary.Write(w, binary.BigEndian, self.KeepAlive)

	var Length uint16 = 0

	if self.Identifier != "" {
		Length = uint16(len(self.Identifier))
	}
	binary.Write(w, binary.BigEndian, Length)
	if Length > 0 {
		w.Write([]byte(self.Identifier))
	}

	if (int(self.Flag)&0x04 > 0) && self.Will != nil {
		self.Will.WriteTo(w)
	}

	if int(self.Flag)&0x80 > 0 {
		Length = uint16(len(self.UserName))
		err = binary.Write(w, binary.BigEndian, Length)
		w.Write([]byte(self.UserName))
	}
	if int(self.Flag)&0x40 > 0 {
		Length = uint16(len(self.Password))
		err = binary.Write(w, binary.BigEndian, Length)
		w.Write([]byte(self.Password))
	}
	return int64(size), nil
}

func (self *ConnectMessage) encode() ([]byte, int, error) {
	var headerLength uint16 = uint16(len(self.Magic))
	var size int = 0

	buffer := bytes.NewBuffer(nil)
	err := binary.Write(buffer, binary.BigEndian, headerLength)
	if err != nil {
		fmt.Printf("1Error: %s\n", err)
	}
	buffer.Write(self.Magic)
	size += 2 + len(self.Magic)

	if self.CleanSession {
		self.Flag |= 0x02
	}
	if self.Will != nil {
		self.Flag |= 0x04
	}
	if len(self.UserName) > 0 {
		self.Flag |= 0x80
	}
	if len(self.Password) > 0 {
		self.Flag |= 0x40
	}

	binary.Write(buffer, binary.BigEndian, self.Version)
	binary.Write(buffer, binary.BigEndian, self.Flag)
	binary.Write(buffer, binary.BigEndian, self.KeepAlive)
	size += 1 + 1 + 2

	var Length uint16 = 0
	if self.Identifier != "" {
		Length = uint16(len(self.Identifier))
	}
	binary.Write(buffer, binary.BigEndian, Length)
	if Length > 0 {
		buffer.Write([]byte(self.Identifier))
	}
	size += 2 + int(Length)

	if (int(self.Flag)&0x04 > 0) && self.Will != nil {
		raw_will, will_size, err := self.Will.encode()
		if err != nil {
		}
		buffer.Write(raw_will)
		size += will_size
	}

	if int(self.Flag)&0x80 > 0 {
		Length = uint16(len(self.UserName))
		err = binary.Write(buffer, binary.BigEndian, Length)
		buffer.Write([]byte(self.UserName))
		size += 2 + int(Length)
	}

	if int(self.Flag)&0x40 > 0 {
		Length = uint16(len(self.Password))
		err = binary.Write(buffer, binary.BigEndian, Length)
		buffer.Write([]byte(self.Password))
		size += 2 + int(Length)
	}

	return buffer.Bytes(), size, nil
}

func (self *ConnectMessage) decode(reader io.Reader) error {
	var Length uint16

	offset := 0
	buffer := make([]byte, self.FixedHeader.RemainingLength)

	w := bytes.NewBuffer(buffer)
	w.Reset()
	io.CopyN(w, reader, int64(self.FixedHeader.RemainingLength))

	buffer = w.Bytes()
	reader = bytes.NewReader(buffer)

	binary.Read(reader, binary.BigEndian, &Length)
	offset += 2

	self.Magic = buffer[offset : offset+int(Length)]
	offset += int(Length)

	nr := bytes.NewReader(buffer[offset:])
	binary.Read(nr, binary.BigEndian, &self.Version)
	binary.Read(nr, binary.BigEndian, &self.Flag)
	binary.Read(nr, binary.BigEndian, &self.KeepAlive)
	offset += 1 + 1 + 2

	// order Client ClientIdentifier, Will Topic, Will Message, User Name, Password
	var ClientIdentifierLength uint16
	binary.Read(nr, binary.BigEndian, &ClientIdentifierLength)
	offset += 2

	if ClientIdentifierLength > 0 {
		self.Identifier = string(buffer[offset : offset+int(ClientIdentifierLength)])
		offset += int(ClientIdentifierLength)
	}

	if int(self.Flag)&0x04 > 0 {
		will := &WillMessage{}

		nr := bytes.NewReader(buffer[offset:])
		binary.Read(nr, binary.BigEndian, &ClientIdentifierLength)
		offset += 2
		will.Topic = string(buffer[offset : offset+int(ClientIdentifierLength)])
		offset += int(ClientIdentifierLength)

		nr = bytes.NewReader(buffer[offset:])
		binary.Read(nr, binary.BigEndian, &ClientIdentifierLength)
		offset += 2
		will.Message = string(buffer[offset : offset+int(ClientIdentifierLength)])
		offset += int(ClientIdentifierLength)

		if int(self.Flag)&0x32 > 0 {
			will.Retain = true
		}

		q := (int(self.Flag) >> 3)

		if q & 0x02 > 0{
			will.Qos = 2
		} else if q & 0x01 > 0{
			will.Qos = 1
		}
		self.Will = will
	}

	if int(self.Flag)&0x80 > 0 {
		nr := bytes.NewReader(buffer[offset:])
		binary.Read(nr, binary.BigEndian, &Length)
		offset += 2
		self.UserName = string(buffer[offset : offset+int(Length)])
		offset += int(Length)
	}

	if int(self.Flag)&0x40 > 0 {
		nr := bytes.NewReader(buffer[offset:])
		offset += 2
		binary.Read(nr, binary.BigEndian, &Length)
		self.Password = string(buffer[offset : offset+int(Length)])
		offset += int(Length)
	}

	if int(self.Flag)&0x02 > 0 {
		self.CleanSession = true
	}

	return nil
}

func (self *ConnectMessage) String() string {
	b, _ := json.Marshal(self)
	return string(b)
}
