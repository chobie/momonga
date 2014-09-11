// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"errors"
)

func NewConnectMessage() *ConnectMessage {
	message := &ConnectMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_CONNECT,
			Dupe: false,
			QosLevel: 0,
		},
		Magic: []byte("MQTT"),
		Version: 4,
	}
	return message
}

func NewSubscribeMessage() *SubscribeMessage {
	message := &SubscribeMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_SUBSCRIBE,
			Dupe: false,
			QosLevel: 1, // Must be 1
		},
	}
	return message
}

func NewUnsubscribeMessage() *UnsubscribeMessage {
	message := &UnsubscribeMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_UNSUBSCRIBE,
			Dupe: false,
			QosLevel: 1, // Must be 1
		},
	}
	return message
}

func NewUnsubackMessage() *UnsubackMessage {
	message := &UnsubackMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_UNSUBACK,
			Dupe: false,
			QosLevel: 0,
		},
	}
	return message
}

func NewSubackMessage() *SubackMessage {
	message := &SubackMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_SUBACK,
			Dupe: false,
			QosLevel: 0,
		},
	}

	return message
}


func NewPublishMessage() *PublishMessage {
	message := &PublishMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_PUBLISH,
			Dupe: false,
			QosLevel: 0,
		},
	}
	return message
}

func NewPubackMessage() *PubackMessage{
	message := &PubackMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_PUBACK,
			Dupe: false,
			QosLevel: 0,
		},
	}

	return message
}

func NewPubrecMessage() *PubrecMessage{
	message := &PubrecMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_PUBREC,
			Dupe: false,
			QosLevel: 0,
		},
	}

	return message
}

func NewPubrelMessage() *PubrelMessage{
	message := &PubrelMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_PUBREL,
			Dupe: false,
			QosLevel: 1,  // Must be 1
		},
	}

	return message
}

func NewPubcompMessage() *PubcompMessage{
	message := &PubcompMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_PUBCOMP,
			Dupe: false,
			QosLevel: 0,
		},
	}

	return message
}

func NewPingreqMessage() *PingreqMessage {
	message := &PingreqMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_PINGREQ,
		},
	}
	return message
}

func NewPingrespMessage() *PingrespMessage {
	message := &PingrespMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_PINGRESP,
		},
	}
	return message
}

func NewDisconnectMessage() *DisconnectMessage{
	message := &DisconnectMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_DISCONNECT,
		},
	}
	return message
}

func NewConnackMessage() *ConnackMessage {
	message := &ConnackMessage{
		FixedHeader: FixedHeader{
			Type: PACKET_TYPE_CONNACK,
			QosLevel: 0,
		},
	}
	return message
}

func CopyPublishMessage(msg *PublishMessage) (*PublishMessage, error) {
	v, err := CopyMessage(msg)
	if err != nil {
		return nil, err
	}

	return v.(*PublishMessage), nil
}

func CopyMessage(msg Message) (Message, error) {
	var result Message

	switch (msg.GetType()) {
	case PACKET_TYPE_PUBLISH:
		t := msg.(*PublishMessage)
		c := NewPublishMessage()
		c.Payload = t.Payload
		c.TopicName = t.TopicName
		c.PacketIdentifier = t.PacketIdentifier
		c.Opaque = t.Opaque
		c.FixedHeader.Type = t.FixedHeader.Type
		c.FixedHeader.Dupe = t.FixedHeader.Dupe
		c.FixedHeader.QosLevel = t.FixedHeader.QosLevel
		c.FixedHeader.Retain = t.FixedHeader.Retain
		c.FixedHeader.RemainingLength = t.FixedHeader.RemainingLength
		result = c
		break
	default:
		return nil, errors.New("hoge")
	}
	return result, nil
}

// TODO: このアホっぽい感じどうにかしたいなー
// TODO: 読み込んだサイズ返す
func ParseMessage(reader io.Reader, max_length int) (Message, error) {
	var message Message

	header := FixedHeader{}
	err := header.decode(reader)
	if err != nil {
		return nil, err
	}

	if max_length > 0 && header.RemainingLength > max_length {
		return nil, errors.New(fmt.Sprintf("Payload exceedes server limit. %d bytes", header.RemainingLength))
	}

	switch header.GetType() {
		case PACKET_TYPE_CONNECT:
			mm := &ConnectMessage{
				FixedHeader: header,
			}
			mm.decode(reader)
			message = mm
	case PACKET_TYPE_CONNACK:
		mm := &ConnackMessage{
			FixedHeader: header,
		}
		mm.decode(reader)
		message = mm
	case PACKET_TYPE_PUBLISH:
		mm := &PublishMessage{
			FixedHeader: header,
		}
		mm.decode(reader)
		message = mm
	case PACKET_TYPE_DISCONNECT:
		mm := &DisconnectMessage{
			FixedHeader: header,
		}
		message = mm
	case PACKET_TYPE_SUBSCRIBE:
		mm := &SubscribeMessage{
			FixedHeader: header,
		}
		mm.decode(reader)
		message = mm
	case PACKET_TYPE_SUBACK:
		mm := &SubackMessage{
			FixedHeader: header,
		}
		mm.decode(reader)
		message = mm
	case PACKET_TYPE_UNSUBSCRIBE:
		mm := &UnsubscribeMessage{
			FixedHeader: header,
		}
		mm.decode(reader)
		message = mm
	case PACKET_TYPE_UNSUBACK:
		mm := &UnsubackMessage{
			FixedHeader: header,
		}
		mm.decode(reader)
		message = mm
	case PACKET_TYPE_PINGRESP:
		mm := &PingrespMessage{
			FixedHeader: header,
		}
		message = mm
	case PACKET_TYPE_PINGREQ:
		mm := &PingreqMessage{
			FixedHeader: header,
		}
		message = mm
	case PACKET_TYPE_PUBACK:
		mm := &PubackMessage{
			FixedHeader: header,
		}
		mm.decode(reader)
		message = mm
	case PACKET_TYPE_PUBREC:
		mm := &PubrecMessage{
			FixedHeader: header,
		}
		mm.decode(reader)
		message = mm
	case PACKET_TYPE_PUBREL:
		mm := &PubrelMessage{
			FixedHeader: header,
		}
		mm.decode(reader)
		message = mm
	case PACKET_TYPE_PUBCOMP:
		mm := &PubcompMessage{
			FixedHeader: header,
		}
		mm.decode(reader)
		message = mm
	default:
		return nil, errors.New(fmt.Sprintf("Not supported: %d\n", header.GetType()))
	}

	return message, nil
}


func Encode(message Message) ([]byte, error){
	buffer := bytes.NewBuffer(nil)

	switch message.GetType() {
	case PACKET_TYPE_CONNECT:
		connect := message.(*ConnectMessage)
		err := binary.Write(buffer, binary.BigEndian, uint8(connect.Type << 0x4))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		// message encode
		raw_connect, connect_size, err := connect.encode()
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}

		err = binary.Write(buffer, binary.BigEndian, uint8(connect_size))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		buffer.Write(raw_connect)
		break
	case PACKET_TYPE_CONNACK:
		connect := message.(*ConnackMessage)
		err := binary.Write(buffer, binary.BigEndian, uint8(connect.Type << 0x4))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		// message encode
		raw_connect, connect_size, err := connect.encode()
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		err = binary.Write(buffer, binary.BigEndian, uint8(connect_size))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		buffer.Write(raw_connect)
		break
	case PACKET_TYPE_PUBLISH:
		connect := message.(*PublishMessage)
		var flag uint8 = uint8(connect.Type << 0x04)

		// TODO: Dup flag
		if connect.Retain > 0{
			flag |= 0x01
		}

		if connect.QosLevel > 0 {
			if connect.QosLevel == 1 {
				flag |= 0x02
			} else if connect.QosLevel == 2 {
				flag |= 0x04
			}
		}

		err := binary.Write(buffer, binary.BigEndian, flag)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}

		// message encode
		raw_connect, connect_size, err := connect.encode()
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		err = binary.Write(buffer, binary.BigEndian, uint8(connect_size))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		buffer.Write(raw_connect)
		break
	case PACKET_TYPE_SUBSCRIBE:
		connect := message.(*SubscribeMessage)
		err := binary.Write(buffer, binary.BigEndian, uint8(connect.Type << 0x04 | 0x02))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		// message encode
		raw_connect, connect_size, err := connect.encode()
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		err = binary.Write(buffer, binary.BigEndian, uint8(connect_size))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		buffer.Write(raw_connect)
		break

	case PACKET_TYPE_SUBACK:
		connect := message.(*SubackMessage)
		err := binary.Write(buffer, binary.BigEndian, uint8(connect.Type << 0x04))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		// message encode
		raw_connect, connect_size, err := connect.encode()
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		err = binary.Write(buffer, binary.BigEndian, uint8(connect_size))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		buffer.Write(raw_connect)
		break

	case PACKET_TYPE_UNSUBSCRIBE:
		connect := message.(*UnsubscribeMessage)
		err := binary.Write(buffer, binary.BigEndian, uint8(connect.Type << 4 | 0x02))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		// message encode
		raw_connect, connect_size, err := connect.encode()
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		err = binary.Write(buffer, binary.BigEndian, uint8(connect_size))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		buffer.Write(raw_connect)
		break

	case PACKET_TYPE_DISCONNECT:
		var remaining uint8 = 0

		connect := message.(*DisconnectMessage)
		binary.Write(buffer, binary.BigEndian, uint8(connect.Type << 4))
		binary.Write(buffer, binary.BigEndian, remaining)
		break

	case PACKET_TYPE_UNSUBACK:
		connect := message.(*UnsubackMessage)
		binary.Write(buffer, binary.BigEndian, uint8(connect.Type << 4))
		raw, size, err := connect.encode()
		binary.Write(buffer, binary.BigEndian, uint8(size))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		buffer.Write(raw)
		break

	case PACKET_TYPE_PUBACK:
		connect := message.(*PubackMessage)
		binary.Write(buffer, binary.BigEndian, uint8(connect.Type << 4))
		raw, size, err := connect.encode()
		binary.Write(buffer, binary.BigEndian, uint8(size))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		buffer.Write(raw)
		break

	case PACKET_TYPE_PUBREC:
		connect := message.(*PubrecMessage)
		binary.Write(buffer, binary.BigEndian, uint8(connect.Type << 4))
		raw, size, err := connect.encode()
		binary.Write(buffer, binary.BigEndian, uint8(size))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		buffer.Write(raw)
		break

	case PACKET_TYPE_PUBREL:
		connect := message.(*PubrelMessage)
		binary.Write(buffer, binary.BigEndian, uint8(connect.Type << 4 | 0x02))
		raw, size, err := connect.encode()
		binary.Write(buffer, binary.BigEndian, uint8(size))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		buffer.Write(raw)
		break

	case PACKET_TYPE_PUBCOMP:
		connect := message.(*PubcompMessage)
		binary.Write(buffer, binary.BigEndian, uint8(connect.Type << 4))
		raw, size, err := connect.encode()
		binary.Write(buffer, binary.BigEndian, uint8(size))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		buffer.Write(raw)
		break

	case PACKET_TYPE_PINGREQ:
		var remaining uint8 = 0

		connect := message.(*PingreqMessage)
		binary.Write(buffer, binary.BigEndian, uint8(connect.Type << 4))
		binary.Write(buffer, binary.BigEndian, remaining)
		break

	case PACKET_TYPE_PINGRESP:
		var remaining uint8 = 0

		connect := message.(*PingrespMessage)
		binary.Write(buffer, binary.BigEndian, uint8(connect.Type << 4))
		binary.Write(buffer, binary.BigEndian, remaining)
		break

	default:
		fmt.Printf("Not supported message")
	}

	return buffer.Bytes(), nil
}

func Encode2(message Message) ([][]byte, int, error){
	buffer := bytes.NewBuffer(nil)

	switch message.GetType() {
	case PACKET_TYPE_PUBLISH:
		publish := message.(*PublishMessage)
		var flag uint8 = uint8(publish.Type << 0x04)

		// TODO: Dup flag
		if publish.Retain > 0{
			flag |= 0x01
		}

		if publish.QosLevel > 0 {
			if publish.QosLevel == 1 {
				flag |= 0x02
			} else if publish.QosLevel == 2 {
				flag |= 0x04
			}
		}

		err := binary.Write(buffer, binary.BigEndian, flag)
		if err != nil {
			return nil, 0, fmt.Errorf("Error: %s\n", err)
		}

		// message encode
		topic, identifier, payload, size := publish.Encode2()

		err = binary.Write(buffer, binary.BigEndian, uint8(size))
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		header := buffer.Bytes()

		return [][]byte{
			header,
			topic,
			identifier,
			payload,
		}, size, nil

	default:
		fmt.Printf("Not supported message: %s", message.GetTypeAsString())
	}

	return nil, 0, fmt.Errorf("failed:")
}
