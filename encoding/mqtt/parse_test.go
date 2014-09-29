// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"bytes"
	. "gopkg.in/check.v1"
	"strings"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type MySuite struct{}

var _ = Suite(&MySuite{})

func (s *MySuite) TestDecodePublishMessage(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")

	buf := bytes.NewBuffer(nil)
	WriteMessageTo(m, buf)

	ParseMessage(bytes.NewReader(buf.Bytes()), 0)
}

func (s *MySuite) TestPublishMessage(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")
	buf := bytes.NewBuffer(nil)
	_, err := WriteMessageTo(m, buf)

	c.Assert(err, Equals, nil)
}

func (s *MySuite) TestPublishMessage2(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")
	buf := bytes.NewBuffer(nil)
	WriteMessageTo(m, buf)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)
	c.Assert(bytes.Compare(buf.Bytes(), buffer.Bytes()), Equals, 0)
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
	buf := bytes.NewBuffer(nil)
	WriteMessageTo(m, buf)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)
	c.Assert(bytes.Compare(buf.Bytes(), buffer.Bytes()), Equals, 0)
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
	buf := bytes.NewBuffer(nil)
	WriteMessageTo(m, buf)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)
	c.Assert(bytes.Compare(buf.Bytes(), buffer.Bytes()), Equals, 0)
}

func (s *MySuite) TestSubackMessage(c *C) {
	m := NewSubackMessage()
	buf := bytes.NewBuffer(nil)
	WriteMessageTo(m, buf)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)
	c.Assert(bytes.Compare(buf.Bytes(), buffer.Bytes()), Equals, 0)
}

func (s *MySuite) TestUnsubackMessage(c *C) {
	m := NewUnsubackMessage()
	buf := bytes.NewBuffer(nil)
	WriteMessageTo(m, buf)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)
	c.Assert(bytes.Compare(buf.Bytes(), buffer.Bytes()), Equals, 0)
}

func (s *MySuite) TestSubscribeMessage(c *C) {
	m := NewSubscribeMessage()
	m.Payload = append(m.Payload, SubscribePayload{TopicPath: "/debug", RequestedQos: 2})
	buf := bytes.NewBuffer(nil)
	WriteMessageTo(m, buf)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)

	c.Assert(bytes.Compare(buf.Bytes(), buffer.Bytes()), Equals, 0)
}

func (s *MySuite) TestConnackMessage(c *C) {
	m := NewConnackMessage()
	buf := bytes.NewBuffer(nil)
	WriteMessageTo(m, buf)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)
	c.Assert(bytes.Compare(buf.Bytes(), buffer.Bytes()), Equals, 0)
}

func (s *MySuite) TestUnsubscribeMessage(c *C) {
	m := NewUnsubscribeMessage()
	m.Payload = append(m.Payload, SubscribePayload{TopicPath: "/debug", RequestedQos: 2})
	buf := bytes.NewBuffer(nil)
	WriteMessageTo(m, buf)

	buffer := bytes.NewBuffer(nil)
	m.WriteTo(buffer)

	c.Assert(bytes.Compare(buf.Bytes(), buffer.Bytes()), Equals, 0)
}

func (s *MySuite) TestConnectMessage(c *C) {
	msg := NewConnectMessage()
	msg.Magic = []byte("MQTT")
	msg.Version = uint8(4)
	msg.Identifier = "debug"
	msg.CleanSession = true
	msg.KeepAlive = uint16(10)

	buf := bytes.NewBuffer(nil)
	WriteMessageTo(msg, buf)

	buffer := bytes.NewBuffer(nil)
	msg.WriteTo(buffer)
	c.Assert(bytes.Compare(buf.Bytes(), buffer.Bytes()), Equals, 0)
}

func (s *MySuite) TestConnectWillMessage(c *C) {
	msg := NewConnectMessage()
	msg.Magic = []byte("MQTT")
	msg.Version = uint8(4)
	msg.Identifier = "debug"
	msg.CleanSession = true
	msg.KeepAlive = uint16(10)
	msg.Will = &WillMessage{
		Topic:   "/debug",
		Message: "Dead",
		Retain:  false,
		Qos:     1,
	}

	buf := bytes.NewBuffer(nil)
	WriteMessageTo(msg, buf)

	ParseMessage(bytes.NewReader(buf.Bytes()), 0)
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

func (s *MySuite) TestEncodeLargePublishMessage(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte(strings.Repeat("a", 1024))

	buf := bytes.NewBuffer(nil)
	_, e := WriteMessageTo(m, buf)
	c.Assert(e, Equals, nil)

	_, e = ParseMessage(bytes.NewReader(buf.Bytes()), 0)
	c.Assert(e, Equals, nil)
}

func (s *MySuite) BenchmarkEncode(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")
	buf := bytes.NewBuffer(nil)

	for i := 0; i < c.N; i++ {
		buf.Reset()
		WriteMessageTo(m, buf)
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
	buf := bytes.NewBuffer(nil)
	WriteMessageTo(m, buf)

	for i := 0; i < c.N; i++ {
		ParseMessage(bytes.NewReader(buf.Bytes()), 0)
	}
}

func (s *MySuite) TestParseConnectMessage(c *C) {
	msg := NewConnectMessage()
	msg.Magic = []byte("MQTT")
	msg.Version = uint8(4)
	msg.UserName = []byte("hoge")
	msg.Password = []byte("huga")
	msg.Identifier = "debug"
	msg.CleanSession = true
	msg.KeepAlive = uint16(10)
	msg.Will = &WillMessage{
		Topic:   "debug",
		Message: "he",
	}
	buf := bytes.NewBuffer(nil)
	WriteMessageTo(msg, buf)

	ParseMessage(bytes.NewReader(buf.Bytes()), 0)

	//c.Assert(bytes.Compare(a, buffer.Bytes()), Equals, 0)
}

func (s *MySuite) BenchmarkParseConnectMessage(c *C) {
	msg := NewConnectMessage()
	msg.Magic = []byte("MQTT")
	msg.Version = uint8(4)
	msg.UserName = []byte("hoge")
	msg.Password = []byte("huga")
	msg.Identifier = "debug"
	msg.CleanSession = true
	msg.KeepAlive = uint16(10)
	msg.Will = &WillMessage{
		Topic:   "debug",
		Message: "he",
	}
	buf := bytes.NewBuffer(nil)
	WriteMessageTo(msg, buf)

	for i := 0; i < c.N; i++ {
		ParseMessage(bytes.NewReader(buf.Bytes()), 0)
	}
}
