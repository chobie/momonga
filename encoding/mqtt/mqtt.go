// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"errors"
	"fmt"
	"io"
)

type ParseError struct {
	reason string
}

func (self ParseError) Error() string {
	return self.reason
}

func NewConnectMessage() *ConnectMessage {
	message := &ConnectMessage{
		FixedHeader: FixedHeader{
			Type:     PACKET_TYPE_CONNECT,
			Dupe:     false,
			QosLevel: 0,
		},
		Magic:   []byte("MQTT"),
		Version: 4,
	}
	return message
}

func NewSubscribeMessage() *SubscribeMessage {
	message := &SubscribeMessage{
		FixedHeader: FixedHeader{
			Type:     PACKET_TYPE_SUBSCRIBE,
			Dupe:     false,
			QosLevel: 1, // Must be 1
		},
	}
	return message
}

func NewUnsubscribeMessage() *UnsubscribeMessage {
	message := &UnsubscribeMessage{
		FixedHeader: FixedHeader{
			Type:     PACKET_TYPE_UNSUBSCRIBE,
			Dupe:     false,
			QosLevel: 1, // Must be 1
		},
	}
	return message
}

func NewUnsubackMessage() *UnsubackMessage {
	message := &UnsubackMessage{
		FixedHeader: FixedHeader{
			Type:     PACKET_TYPE_UNSUBACK,
			Dupe:     false,
			QosLevel: 0,
		},
	}
	return message
}

func NewSubackMessage() *SubackMessage {
	message := &SubackMessage{
		FixedHeader: FixedHeader{
			Type:     PACKET_TYPE_SUBACK,
			Dupe:     false,
			QosLevel: 0,
		},
	}

	return message
}

func NewPublishMessage() *PublishMessage {
	message := &PublishMessage{
		FixedHeader: FixedHeader{
			Type:     PACKET_TYPE_PUBLISH,
			Dupe:     false,
			QosLevel: 0,
		},
	}
	return message
}

func NewPubackMessage() *PubackMessage {
	message := &PubackMessage{
		FixedHeader: FixedHeader{
			Type:     PACKET_TYPE_PUBACK,
			Dupe:     false,
			QosLevel: 0,
		},
	}

	return message
}

func NewPubrecMessage() *PubrecMessage {
	message := &PubrecMessage{
		FixedHeader: FixedHeader{
			Type:     PACKET_TYPE_PUBREC,
			Dupe:     false,
			QosLevel: 0,
		},
	}

	return message
}

func NewPubrelMessage() *PubrelMessage {
	message := &PubrelMessage{
		FixedHeader: FixedHeader{
			Type:     PACKET_TYPE_PUBREL,
			Dupe:     false,
			QosLevel: 1, // Must be 1
		},
	}

	return message
}

func NewPubcompMessage() *PubcompMessage {
	message := &PubcompMessage{
		FixedHeader: FixedHeader{
			Type:     PACKET_TYPE_PUBCOMP,
			Dupe:     false,
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

func NewDisconnectMessage() *DisconnectMessage {
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
			Type:     PACKET_TYPE_CONNACK,
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

	switch msg.GetType() {
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
	var err error
	header := FixedHeader{}

	err = header.decode(reader)
	if err != nil {
		return nil, err
	}

	if max_length > 0 && header.RemainingLength > max_length {
		return nil, errors.New(fmt.Sprintf("Payload exceedes limit. %d bytes", header.RemainingLength))
	}

	switch header.GetType() {
	case PACKET_TYPE_CONNECT:
		mm := &ConnectMessage{
			FixedHeader: header,
		}
		err = mm.decode(reader)
		message = mm
	case PACKET_TYPE_CONNACK:
		mm := &ConnackMessage{
			FixedHeader: header,
		}
		err = mm.decode(reader)
		message = mm
	case PACKET_TYPE_PUBLISH:
		mm := &PublishMessage{
			FixedHeader: header,
		}
		err = mm.decode(reader)
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
		err = mm.decode(reader)
		message = mm
	case PACKET_TYPE_SUBACK:
		mm := &SubackMessage{
			FixedHeader: header,
		}
		err = mm.decode(reader)
		message = mm
	case PACKET_TYPE_UNSUBSCRIBE:
		mm := &UnsubscribeMessage{
			FixedHeader: header,
		}
		err = mm.decode(reader)
		message = mm
	case PACKET_TYPE_UNSUBACK:
		mm := &UnsubackMessage{
			FixedHeader: header,
		}
		err = mm.decode(reader)
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
		err = mm.decode(reader)
		message = mm
	case PACKET_TYPE_PUBREC:
		mm := &PubrecMessage{
			FixedHeader: header,
		}
		err = mm.decode(reader)
		message = mm
	case PACKET_TYPE_PUBREL:
		mm := &PubrelMessage{
			FixedHeader: header,
		}
		err = mm.decode(reader)
		message = mm
	case PACKET_TYPE_PUBCOMP:
		mm := &PubcompMessage{
			FixedHeader: header,
		}
		err = mm.decode(reader)
		message = mm
	default:
		return nil, &ParseError{fmt.Sprintf("Not supported: %d\n", header.GetType())}
	}

	if err != nil {
		return nil, err
	}

	return message, nil
}

func WriteMessageTo(message Message, w io.Writer) (int64, error) {
	var written int64
	var e error

	switch message.GetType() {
	case PACKET_TYPE_CONNECT:
		m := message.(*ConnectMessage)
		written, e = m.WriteTo(w)
	case PACKET_TYPE_CONNACK:
		m := message.(*ConnackMessage)
		written, e = m.WriteTo(w)
	case PACKET_TYPE_PUBLISH:
		m := message.(*PublishMessage)
		written, e = m.WriteTo(w)
	case PACKET_TYPE_SUBSCRIBE:
		m := message.(*SubscribeMessage)
		written, e = m.WriteTo(w)
	case PACKET_TYPE_SUBACK:
		m := message.(*SubackMessage)
		written, e = m.WriteTo(w)
	case PACKET_TYPE_UNSUBSCRIBE:
		m := message.(*UnsubscribeMessage)
		written, e = m.WriteTo(w)
	case PACKET_TYPE_DISCONNECT:
		m := message.(*DisconnectMessage)
		written, e = m.WriteTo(w)
	case PACKET_TYPE_UNSUBACK:
		m := message.(*UnsubackMessage)
		written, e = m.WriteTo(w)
	case PACKET_TYPE_PUBACK:
		m := message.(*PubackMessage)
		written, e = m.WriteTo(w)
	case PACKET_TYPE_PUBREC:
		m := message.(*PubrecMessage)
		written, e = m.WriteTo(w)
	case PACKET_TYPE_PUBREL:
		m := message.(*PubrelMessage)
		written, e = m.WriteTo(w)
	case PACKET_TYPE_PUBCOMP:
		m := message.(*PubcompMessage)
		written, e = m.WriteTo(w)
	case PACKET_TYPE_PINGREQ:
		m := message.(*PingreqMessage)
		written, e = m.WriteTo(w)
	case PACKET_TYPE_PINGRESP:
		m := message.(*PingrespMessage)
		written, e = m.WriteTo(w)
	default:
		fmt.Printf("Not supported message")
	}

	return written, e
}
