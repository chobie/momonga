// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"bytes"
	"fmt"
	. "gopkg.in/check.v1"
	"testing"
	//	"encoding/hex"
)

func Test(t *testing.T) { TestingT(t) }

type MySuite struct{}

var _ = Suite(&MySuite{})

func (s *MySuite) TestDecodePublishMessage(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")
	b, _ := Encode(m)

	x, _ := ParseMessage(bytes.NewReader(b), 0)
	xx := x.(*PublishMessage)
	fmt.Printf("%s\n", xx)

}

func (s *MySuite) TestPublishMessage(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")
	_, err := Encode(m)

	c.Assert(err, Equals, nil)
}

func (s *MySuite) TestPublishMessage2(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")
	b, _ := Encode(m)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)
	c.Assert(bytes.Compare(b, buffer.Bytes()), Equals, 0)
}

func (s *MySuite) BenchmarkWriteToPublishMessage(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")

	b := make([]byte, 256)
	buffer := bytes.NewBuffer(b)
	for i := 0; i < c.N; i++ {
		buffer.Reset()
		m.WriteTo(buffer)
	}
}

func (s *MySuite) TestPubrecMessage(c *C) {
	m := NewPubrecMessage()
	b, _ := Encode(m)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)
	c.Assert(bytes.Compare(b, buffer.Bytes()), Equals, 0)
}

func (s *MySuite) BenchmarkPubrecMessage(c *C) {
	m := NewPubrecMessage()

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)

	for i := 0; i < c.N; i++ {
		buffer.Reset()
		m.WriteTo(buffer)
	}
}

func (s *MySuite) TestPubrelMessage(c *C) {
	m := NewPubrelMessage()
	b, _ := Encode(m)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)
	c.Assert(bytes.Compare(b, buffer.Bytes()), Equals, 0)
}

func (s *MySuite) TestSubackMessage(c *C) {
	m := NewSubackMessage()
	b, _ := Encode(m)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)
	c.Assert(bytes.Compare(b, buffer.Bytes()), Equals, 0)
}

func (s *MySuite) TestUnsubackMessage(c *C) {
	m := NewUnsubackMessage()
	b, _ := Encode(m)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)
	c.Assert(bytes.Compare(b, buffer.Bytes()), Equals, 0)
}

func (s *MySuite) TestSubscribeMessage(c *C) {
	m := NewSubscribeMessage()
	m.Payload = append(m.Payload, SubscribePayload{TopicPath: "/debug", RequestedQos: 2})
	a, _ := Encode(m)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)

	c.Assert(bytes.Compare(a, buffer.Bytes()), Equals, 0)
}

func (s *MySuite) TestConnackMessage(c *C) {
	m := NewConnackMessage()
	b, _ := Encode(m)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)
	c.Assert(bytes.Compare(b, buffer.Bytes()), Equals, 0)
}

func (s *MySuite) TestUnsubscribeMessage(c *C) {
	m := NewUnsubscribeMessage()
	m.Payload = append(m.Payload, SubscribePayload{TopicPath: "/debug", RequestedQos: 2})
	a, _ := Encode(m)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)

	c.Assert(bytes.Compare(a, buffer.Bytes()), Equals, 0)
}

func (s *MySuite) TestConnectMessage(c *C) {
	msg := NewConnectMessage()
	msg.Magic = []byte("MQTT")
	msg.Version = uint8(4)
	msg.Identifier = "debug"
	msg.CleanSession = true
	msg.KeepAlive = uint16(10)

	a, _ := Encode(msg)

	buffer := bytes.NewBuffer(nil)
	msg.WriteTo(buffer)
	c.Assert(bytes.Compare(a, buffer.Bytes()), Equals, 0)
}

func (s *MySuite) BenchmarkConnectMessage(c *C) {
	msg := NewConnectMessage()
	msg.Magic = []byte("MQTT")
	msg.Version = uint8(4)
	msg.Identifier = "debug"
	msg.CleanSession = true
	msg.KeepAlive = uint16(10)

	buffer := bytes.NewBuffer(nil)
	for i := 0; i < c.N; i++ {
		buffer.Reset()
		msg.WriteTo(buffer)
	}
}

func (s *MySuite) BenchmarkEncode(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")

	for i := 0; i < c.N; i++ {
		Encode(m)
	}
}

func (s *MySuite) BenchmarkParsePublishMessage(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")

	for i := 0; i < c.N; i++ {
		ParseMessage(bytes.NewReader(nil), 0)
	}
}

func (s *MySuite) BenchmarkParseSubscribe(c *C) {
	m := NewSubscribeMessage()
	m.Payload = append(m.Payload, SubscribePayload{TopicPath: "/debug"})
	m.PacketIdentifier = 1
	b, _ := Encode(m)

	for i := 0; i < c.N; i++ {
		ParseMessage(bytes.NewReader(b), 0)
	}
}

func (s *MySuite) TestParseConnectMessage(c *C) {
	msg := NewConnectMessage()
	msg.Magic = []byte("MQTT")
	msg.Version = uint8(4)
	msg.UserName = "hoge"
	msg.Password = "huga"
	msg.Identifier = "debug"
	msg.CleanSession = true
	msg.KeepAlive = uint16(10)
	msg.Will = &WillMessage{
		Topic:   "debug",
		Message: "he",
	}
	a, _ := Encode(msg)

	ParseMessage(bytes.NewReader(a), 0)

	//c.Assert(bytes.Compare(a, buffer.Bytes()), Equals, 0)
}

func (s *MySuite) BenchmarkParseConnectMessage(c *C) {
	msg := NewConnectMessage()
	msg.Magic = []byte("MQTT")
	msg.Version = uint8(4)
	msg.UserName = "hoge"
	msg.Password = "huga"
	msg.Identifier = "debug"
	msg.CleanSession = true
	msg.KeepAlive = uint16(10)
	msg.Will = &WillMessage{
		Topic:   "debug",
		Message: "he",
	}
	a, _ := Encode(msg)

	for i := 0; i < c.N; i++ {
		ParseMessage(bytes.NewReader(a), 0)
	}
}
