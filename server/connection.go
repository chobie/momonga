package server

import (
	"bytes"
	"github.com/chobie/momonga/encoding/mqtt"
	"github.com/chobie/momonga/util"
	"net"
)

type State int32

//   ConnectionState: Idle, Send, Receive and Handshake?
const (
	STATE_CONNECTED State = iota
	STATE_ACCEPTED
	STATE_IDLE
	STATE_DETACHED
	STATE_SEND
	STATE_RECEIVE
	STATE_SHUTDOWN
)

type ConnectionError struct {
	s string
}

func (e *ConnectionError) Error() string {
	return e.s
}

type ConnectionResetError struct {
	s string
}

func (e *ConnectionResetError) Error() string {
	return e.s
}

type Connection interface {
	WriteMessage(request mqtt.Message) error
	WriteMessageQueue(request mqtt.Message)
	Close()
	SetState(State)
	GetState() State
	ResetState()
	ReadMessage() (mqtt.Message, error)
	GetSocket() net.Conn
	SetSocket(net.Conn)
	ClearBuffer()
	GetAddress() net.Addr
	Write(reader *bytes.Reader) error
	IsAlived() bool
	SetWillMessage(mqtt.WillMessage)
	GetWillMessage() *mqtt.WillMessage
	HasWillMessage() bool
	GetOutGoingTable() *util.MessageTable
	GetSubscribedTopicQos(string) int
	GetSubscribedTopics() map[string]int
	AppendSubscribedTopic(string, int)
	RemoveSubscribedTopic(string)
	SetKeepaliveInterval(int)
	GetId() string
	DisableClearSession()
	ShouldClearSession() bool
}
