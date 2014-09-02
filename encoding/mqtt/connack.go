package mqtt

import (
	"bytes"
	"encoding/binary"
	"io"
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
