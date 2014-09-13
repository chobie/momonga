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

type WillMessage struct {
	Qos uint8 `json:"qos"`
	Topic string `json:"topic"`
	Message string `json:"message"`
	Retain bool `json:retain`
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


func (self *WillMessage) WriteTo(w io.Writer) (int64, error) {
	var topic_length uint16
	var size int = 0

	topic_length = uint16(len(self.Topic))
	err := binary.Write(w, binary.BigEndian, topic_length)
	w.Write([]byte(self.Topic))
	size += 2 + int(topic_length)

	message_length := uint16(len(self.Message))
	err = binary.Write(w, binary.BigEndian, message_length)
	w.Write([]byte(self.Message))
	size += 2 + int(message_length)

	return int64(size), err
}

func (self *WillMessage) Size() (int){
	var size int = 0

	topic_length := uint16(len(self.Topic))
	size += 2 + int(topic_length)

	message_length := uint16(len(self.Message))
	size += 2 + int(message_length)

	return size
}

func (self *WillMessage) String() string {
	b, _ := json.Marshal(self)
	return string(b)
}
