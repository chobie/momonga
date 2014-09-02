package mqtt

import (
	"bytes"
	"encoding/binary"
	"io"
)

type PubackMessage struct {
	FixedHeader
	Identifier uint16
}

func (self *PubackMessage) encode() ([]byte, int, error) {
	buffer := bytes.NewBuffer(nil)
	binary.Write(buffer, binary.BigEndian, self.Identifier)
	return buffer.Bytes(), 2, nil
}

func (self *PubackMessage) decode(reader io.Reader) error {
	binary.Read(reader, binary.BigEndian, &self.Identifier)
	return nil
}
