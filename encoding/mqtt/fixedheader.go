package mqtt

import (
	"encoding/binary"
	"io"
	log "github.com/chobie/momonga/logger"
)

type FixedHeader struct {
	Type     MessageType
	Dupe     bool
	QosLevel int
	Retain   int
	RemainingLength int
}

func (self *FixedHeader) GetType() MessageType {
	return self.Type
}

func (self *FixedHeader) decode(reader io.Reader) error {
	var FirstByte uint8
	err := binary.Read(reader, binary.BigEndian, &FirstByte)
	if err != nil {
		log.Debug("Error: %s\n", err)
		return err
	}

	mt := FirstByte >> 4
	flag := FirstByte & 0x0f

	length, _ := ReadVarint(reader)
	self.Type = MessageType(mt)
	self.Dupe = ((flag & 0x08) > 0)

	if (flag &0x01) > 0 {
		self.Retain = 1
	}

	if (flag & 0x04) > 0 {
		self.QosLevel = 2
	} else if (flag & 0x02) > 0 {
		self.QosLevel = 1
	}
	self.RemainingLength = length

	return nil
}

