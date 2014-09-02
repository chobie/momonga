package mqtt

import (
	"bytes"
	"encoding/binary"
	"io"
)

type UnsubackMessage struct {
	FixedHeader
	Identifier uint16
}

func (self *UnsubackMessage) encode() ([]byte, int, error) {
	buffer := bytes.NewBuffer(nil)
	binary.Write(buffer, binary.BigEndian, self.Identifier)
	return buffer.Bytes(), 2, nil
}

func (self *UnsubackMessage) decode(reader io.Reader) error {
	binary.Read(reader, binary.BigEndian, &self.Identifier)
	return nil
}
