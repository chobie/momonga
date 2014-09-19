package server

import (
	"bytes"
	. "github.com/chobie/momonga/common"
	"github.com/chobie/momonga/configuration"
	codec "github.com/chobie/momonga/encoding/mqtt"
	log "github.com/chobie/momonga/logger"
	. "gopkg.in/check.v1"
	"net"
	"os"
	"testing"
	"time"
)

// mock
type MockConnection struct {
	bytes.Buffer
	Closed bool
	Local  mockAddr
	Remote mockAddr
	Type   int
}

type mockAddr struct {
}

func (m *mockAddr) Network() string {
	return "debug"
}

func (m *mockAddr) String() string {
	return "debug"
}

func (m *MockConnection) Close() error {
	m.Closed = true
	return nil
}

func (m *MockConnection) LocalAddr() net.Addr {
	return &m.Local
}

func (m *MockConnection) RemoteAddr() net.Addr {
	return &m.Remote
}

func (m *MockConnection) SetDeadline(t time.Time) error {
	return nil
}

func (m *MockConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *MockConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

func CreateEngine() *Momonga {
	return NewMomonga(configuration.DefaultConfiguration())
}

func Test(t *testing.T) { TestingT(t) }

type EngineSuite struct{}

var _ = Suite(&EngineSuite{})

func (s *EngineSuite) SetUpSuite(c *C) {
	os.Remove("/Users/chobie/src/momonga/socket")
}

func (s *EngineSuite) TearDownSuite(c *C) {
}

func (s *EngineSuite) TestBasic(c *C) {
	log.SetupLogging("error", "stdout")

	// This test introduce how to setup custom MQTT server

	// 1) You need to setup Momonga engine like this.
	engine := CreateEngine()
	// 2) Running Engine
	go engine.Run()

	// 3) engine expects net.Conn (see MockConnection)
	mock := &MockConnection{}
	conn := NewMyConnection(nil)
	conn.SetMyConnection(mock)
	conn.SetId("debug")

	// 4) setup handler. This handler implementation is example.
	// you can customize behaviour with On method.
	hndr := NewHandler(conn, engine)
	conn.SetOpaque(hndr)

	// 5) Now,
	msg := codec.NewConnectMessage()
	msg.Magic = []byte("MQTT")
	msg.Version = uint8(4)
	msg.Identifier = "debug"
	msg.CleanSession = true
	msg.KeepAlive = uint16(0) // Ping is annoyed at this time

	codec.WriteMessageTo(msg, mock)

	// 6) just call conn.ParseMessage(). then handler will work.
	r, err := conn.ParseMessage()

	c.Assert(err, Equals, nil)
	c.Assert(r.GetType(), Equals, codec.PACKET_TYPE_CONNECT)

	// NOTE: Client turn. don't care this.
	time.Sleep(time.Millisecond)
	r, err = conn.ParseMessage()
	c.Assert(err, Equals, nil)
	c.Assert(r.GetType(), Equals, codec.PACKET_TYPE_CONNACK)

	// Subscribe
	sub := codec.NewSubscribeMessage()
	sub.Payload = append(sub.Payload, codec.SubscribePayload{TopicPath: "/debug"})
	sub.PacketIdentifier = 1

	codec.WriteMessageTo(sub, mock)

	// (Server)
	r, err = conn.ParseMessage()
	c.Assert(err, Equals, nil)
	c.Assert(r.GetType(), Equals, codec.PACKET_TYPE_SUBSCRIBE)
	rsub := r.(*codec.SubscribeMessage)
	c.Assert(rsub.PacketIdentifier, Equals, uint16(1))
	c.Assert(rsub.Payload[0].TopicPath, Equals, "/debug")

	// (Client) suback
	time.Sleep(time.Millisecond)
	r, err = conn.ParseMessage()
	c.Assert(err, Equals, nil)
	c.Assert(r.GetType(), Equals, codec.PACKET_TYPE_SUBACK)

	// (Client) Publish
	pub := codec.NewPublishMessage()
	pub.TopicName = "/debug"
	pub.Payload = []byte("hello")
	codec.WriteMessageTo(pub, mock)

	// (Server)
	r, err = conn.ParseMessage()
	c.Assert(err, Equals, nil)
	c.Assert(r.GetType(), Equals, codec.PACKET_TYPE_PUBLISH)
	rpub := r.(*codec.PublishMessage)
	c.Assert(rpub.TopicName, Equals, "/debug")

	// (Client) received publish message
	time.Sleep(time.Millisecond)
	r, err = conn.ParseMessage()
	rp := r.(*codec.PublishMessage)
	c.Assert(err, Equals, nil)
	c.Assert(r.GetType(), Equals, codec.PACKET_TYPE_PUBLISH)
	c.Assert(rp.TopicName, Equals, "/debug")
	c.Assert(string(rp.Payload), Equals, "hello")

	// okay, now disconnect from server
	dis := codec.NewDisconnectMessage()
	codec.WriteMessageTo(dis, mock)

	r, err = conn.ParseMessage()
	c.Assert(err, Equals, nil)
	c.Assert(r.GetType(), Equals, codec.PACKET_TYPE_DISCONNECT)

	// then, receive EOF from server.
	time.Sleep(time.Millisecond)
	r, err = conn.ParseMessage()
	c.Assert(err, Equals, nil)

	// That's it.
	engine.Terminate()
}

func (s *EngineSuite) BenchmarkSimple(c *C) {
	log.SetupLogging("error", "stdout")

	engine := CreateEngine()
	go engine.Run()

	mock := &MockConnection{}
	conn := NewMyConnection(nil)
	conn.SetMyConnection(mock)
	conn.SetId("debug")

	hndr := NewHandler(conn, engine)
	conn.SetOpaque(hndr)

	msg := codec.NewConnectMessage()
	msg.Magic = []byte("MQTT")
	msg.Version = uint8(4)
	msg.Identifier = "debug"
	msg.CleanSession = true
	msg.KeepAlive = uint16(0) // Ping is annoyed at this time
	codec.WriteMessageTo(msg, mock)

	// 6) just call conn.ParseMessage(). then handler will work.
	r, err := conn.ParseMessage()

	c.Assert(err, Equals, nil)
	c.Assert(r.GetType(), Equals, codec.PACKET_TYPE_CONNECT)

	// NOTE: Client turn. don't care this.
	time.Sleep(time.Millisecond)
	r, err = conn.ParseMessage()
	c.Assert(err, Equals, nil)
	c.Assert(r.GetType(), Equals, codec.PACKET_TYPE_CONNACK)

	for i := 0; i < c.N; i++ {
		// (Client) Publish
		pub := codec.NewPublishMessage()
		pub.TopicName = "/debug"
		pub.Payload = []byte("hello")
		codec.WriteMessageTo(pub, mock)

		// (Server)
		conn.ParseMessage()
	}

	// That's it.
	engine.Terminate()
}
