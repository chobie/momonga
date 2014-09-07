package server

import (
	"github.com/chobie/momonga/encoding/mqtt"
	"github.com/chobie/momonga/util"
)

type State int32

//   ConnectionState: Idle, Send, Receive and Handshake?
const (
	STATE_INIT State = iota
	STATE_CONNECTING
	STATE_CONNECTED
	STATE_ACCEPTED
	STATE_IDLE
	STATE_DETACHED
	STATE_SEND
	STATE_RECEIVE
	STATE_SHUTDOWN
	STATE_CLOSED
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
	Close() error
	SetState(State)
	GetState() State
	ResetState()
	ReadMessage() (mqtt.Message, error)
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
	GetRealId() string
	SetId(string)
	DisableClearSession()
	ShouldClearSession() bool
}
