// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package mqtt

import (
	"testing"
	. "gopkg.in/check.v1"
	"bytes"
)

func Test(t *testing.T) { TestingT(t) }

type MySuite struct{}

var _ = Suite(&MySuite{})

func (s *MySuite) TESTPublishMessage(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")
	_, err := Encode(m)

	c.Assert(err, Equals, nil)
}

func (s *MySuite) TestEncoder2(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")
	m.QosLevel = 1

	t, i, p, _ := m.Encode2()
	t = append(t, i...)
	t = append(t, p...)

	b, _, _ := m.encode()
	c.Assert(bytes.Compare(t, b), Equals, 0)

	m.QosLevel = 0
	t, i, p, _ = m.Encode2()
	t = append(t, i...)
	t = append(t, p...)

	b, _, _ = m.encode()
	c.Assert(bytes.Compare(t, b), Equals, 0)
}

func (s *MySuite) BenchmarkEncoder2(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")
	m.QosLevel = 1

	for i := 0; i < c.N; i++ {
		t, i, p, _ := m.Encode2()

		t = append(t, i...)
		t = append(t, p...)
	}
}

func (s *MySuite) TestEncoder22(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")
	m.QosLevel = 2

	r, _ := Encode2(m)

	a, _ := Encode(m)

	x := []byte{}

	x = append(x, r[0]...)
	x = append(x, r[1]...)
	x = append(x, r[2]...)
	x = append(x, r[3]...)

	c.Assert(bytes.Compare(x, a), Equals, 0)

}


func (s *MySuite) BenchmarkEncode(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")

	for i := 0; i < c.N; i++ {
		Encode(m)
	}
}

func (s *MySuite) BenchmarkParse(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")
	b, _ := Encode(m)

	for i := 0; i < c.N; i++ {
		ParseMessage(bytes.NewReader(b), 0)
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
