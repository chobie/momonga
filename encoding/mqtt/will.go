package mqtt

import (
	"bytes"
	"encoding/binary"
)

type WillMessage struct {
	Qos uint8
	Topic string
	Message string
	Retain bool
}

func (self *WillMessage) encode() ([]byte, int, error){
	var topic_length uint16
	var size int = 0

	buffer := bytes.NewBuffer(nil)

	topic_length = uint16(len(self.Topic))
	err := binary.Write(buffer, binary.BigEndian, topic_length)
	buffer.Write([]byte(self.Topic))
	size += 2 + int(topic_length)

	message_length := uint16(len(self.Message))
	err = binary.Write(buffer, binary.BigEndian, message_length)
	buffer.Write([]byte(self.Message))
	size += 2 + int(message_length)

	return buffer.Bytes(), size, err
}
