package mqtt

import (
	"testing"
	. "gopkg.in/check.v1"
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

func (s *MySuite) BenchmarkLogic(c *C) {
	m := NewPublishMessage()
	m.TopicName = "/debug"
	m.Payload = []byte("Hello World")

	for i := 0; i < 10000; i++ {
		Encode(m)
	}
}
