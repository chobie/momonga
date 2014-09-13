// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

type FixedHeader struct {
	Type            PacketType
	Dupe            bool
	QosLevel        int
	Retain          int
	RemainingLength int
}

func (self *FixedHeader) GetType() PacketType {
	return self.Type
}

func (self *FixedHeader) GetTypeAsString() string {
	switch self.Type {
	case PACKET_TYPE_RESERVED1:
		return "unknown"
	case PACKET_TYPE_CONNECT:
		return "connect"
	case PACKET_TYPE_CONNACK:
		return "connack"
	case PACKET_TYPE_PUBLISH:
		return "publish"
	case PACKET_TYPE_PUBACK:
		return "puback"
	case PACKET_TYPE_PUBREC:
		return "pubrec"
	case PACKET_TYPE_PUBREL:
		return "pubrel"
	case PACKET_TYPE_PUBCOMP:
		return "pubcomp"
	case PACKET_TYPE_SUBSCRIBE:
		return "subscribe"
	case PACKET_TYPE_SUBACK:
		return "suback"
	case PACKET_TYPE_UNSUBSCRIBE:
		return "unsubscribe"
	case PACKET_TYPE_UNSUBACK:
		return "unsuback"
	case PACKET_TYPE_PINGREQ:
		return "pingreq"
	case PACKET_TYPE_PINGRESP:
		return "pingresp"
	case PACKET_TYPE_DISCONNECT:
		return "disconnect"
	case PACKET_TYPE_RESERVED2:
		return "unknown"
	default:
		return "unknown"
	}
}

func (self *FixedHeader) writeTo(length uint8, w io.Writer) (int64, error) {
	var flag uint8 = uint8(self.Type << 0x04)

	if self.Retain > 0 {
		flag |= 0x01
	}

	if self.QosLevel > 0 {
		if self.QosLevel == 1 {
			flag |= 0x02
		} else if self.QosLevel == 2 {
			flag |= 0x04
		}
	}

	if self.Dupe {
		flag |= 0x08
	}

	err := binary.Write(w, binary.BigEndian, flag)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}

	// TODO: これ可変長なのよね
	err = binary.Write(w, binary.BigEndian, uint8(length))
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}

	return int64(2), nil
}

func (self *FixedHeader) decode(reader io.Reader) error {
	var FirstByte uint8
	err := binary.Read(reader, binary.BigEndian, &FirstByte)
	if err != nil {
		return err
	}

	mt := FirstByte >> 4
	flag := FirstByte & 0x0f

	length, _ := ReadVarint(reader)
	self.Type = PacketType(mt)
	self.Dupe = ((flag & 0x08) > 0)

	if (flag & 0x01) > 0 {
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

func (self *FixedHeader) String() string {
	b, _ := json.Marshal(self)
	return string(b)
}
