package mqtt

import (
	"bytes"
	"encoding/binary"
	"io"
	"fmt"
)

type ConnectMessage struct {
	FixedHeader
	Magic        []byte
	Version      uint8
	Flag         uint8
	KeepAlive    uint16
	Identifier   string
	Will         *WillMessage
	CleanSession bool
	UserName     string
	Password     string
}

func (self *ConnectMessage) encode() ([]byte, int, error) {
	var headerLength uint16 = uint16(len(self.Magic))
	var size int = 0

	buffer := bytes.NewBuffer(nil)
	err := binary.Write(buffer, binary.BigEndian, headerLength)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}
	buffer.Write(self.Magic)
	size += 2 + len(self.Magic)

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

	if int(self.Flag) & 0x80 > 0 {
		Length = uint16(len(self.UserName))
		err = binary.Write(buffer, binary.BigEndian, Length)
		buffer.Write([]byte(self.UserName))
		size += 2 + int(Length)
	}

	if int(self.Flag) & 0x40 > 0 {
		Length = uint16(len(self.Password))
		err = binary.Write(buffer, binary.BigEndian, Length)
		buffer.Write([]byte(self.Password))
		size += 2 + int(Length)
	}

	return buffer.Bytes(), size, nil
}

func (self *ConnectMessage) decode(reader io.Reader) error {
	var Length uint16
	binary.Read(reader, binary.BigEndian, &Length)

	buffer := bytes.NewBuffer(self.Magic)
	_, err := io.CopyN(buffer, reader, int64(Length))
	if err != nil {
		fmt.Printf("ERROR")
	}

	self.Magic = buffer.Bytes()
	binary.Read(reader, binary.BigEndian, &self.Version)
	binary.Read(reader, binary.BigEndian, &self.Flag)
	binary.Read(reader, binary.BigEndian, &self.KeepAlive)

	// order Client Identifier, Will Topic, Will Message, User Name, Password
	var IdentifierLength uint16
	binary.Read(reader, binary.BigEndian, &IdentifierLength)
	if IdentifierLength > 0 {
		vv := &bytes.Buffer{}
		_, err := io.CopyN(vv, reader, int64(IdentifierLength))
		if err != nil {
			fmt.Printf("DAM ERROR")
		}
		self.Identifier = string(vv.Bytes())
	}

	if int(self.Flag) & 0x04 > 0 {
		will := &WillMessage{}

		binary.Read(reader, binary.BigEndian, &IdentifierLength)
		vv := &bytes.Buffer{}
		_, err := io.CopyN(vv, reader, int64(IdentifierLength))
		if err != nil {
			fmt.Printf("DAM ERROR")
		}
		will.Topic = string(vv.Bytes())

		binary.Read(reader, binary.BigEndian, &IdentifierLength)
		v2 := &bytes.Buffer{}
		_, err = io.CopyN(v2, reader, int64(IdentifierLength))
		if err != nil {
			fmt.Printf("DAM ERROR")
		}
		will.Message = string(v2.Bytes())

		if int(self.Flag)&0x32 > 0 {
			will.Retain = true
		}
		if int(self.Flag)&0x32 > 0 {
			will.Qos = uint8((int(self.Flag) >> 3 & 0x23))
		}

		self.Will = will
	}

	if int(self.Flag) & 0x80 > 0 {
		binary.Read(reader, binary.BigEndian, &Length)
		vv := &bytes.Buffer{}
		_, err := io.CopyN(vv, reader, int64(Length))
		if err != nil {
			fmt.Printf("ERROR")
		}
		self.UserName = string(vv.Bytes())
	}

	if int(self.Flag) & 0x40 > 0 {
		binary.Read(reader, binary.BigEndian, &Length)
		vv := &bytes.Buffer{}
		_, err := io.CopyN(vv, reader, int64(Length))
		if err != nil {
			fmt.Printf("ERROR")
		}
		self.Password = string(vv.Bytes())
	}

	if int(self.Flag)&0x02 > 0 {
		self.CleanSession = true
	}

	return nil
}
