package server

import (
	. "github.com/chobie/momonga/common"
	"github.com/chobie/momonga/encoding/mqtt"
	. "gopkg.in/check.v1"
)

type DummyPlugSuite struct{}

var _ = Suite(&DummyPlugSuite{})

func (s *DummyPlugSuite) TestDummyPlug(c *C) {
	engine := CreateEngine()
	// 2) Running Engine
	go engine.Run()

	p := NewDummyPlug(engine)
	p.Switch <- true
	mux := NewMmuxConnection()

	mux.SetState(STATE_CONNECTED)
	mux.DisableClearSession()
	mux.SetId(p.GetId())
	mux.Attach(p)
	// Memo: Normally, engine has correct relation ship between mux and iteself. this is only need for test
	engine.Connections[p.GetId()] = mux

	sub := mqtt.NewSubscribeMessage()
	sub.Payload = append(sub.Payload, mqtt.SubscribePayload{
		TopicPath:    "/debug/1",
		RequestedQos: uint8(1),
	})
	sub.Payload = append(sub.Payload, mqtt.SubscribePayload{
		TopicPath:    "/debug/2",
		RequestedQos: uint8(2),
	})

	sub.PacketIdentifier = 1
	engine.Subscribe(sub, mux)

	pub := mqtt.NewPublishMessage()
	pub.TopicName = "/debug/1"
	pub.Payload = []byte("hello")
	pub.QosLevel = 1
	engine.SendPublishMessage(pub)

	pub = mqtt.NewPublishMessage()
	pub.TopicName = "/debug/2"
	pub.Payload = []byte("hello")
	pub.QosLevel = 2

	engine.SendPublishMessage(pub)

	// TODO: How do i test this?
}
