// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package common

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

// TODO: あんまり実情にあってないのでみなおそう
type Connection interface {
	//WriteMessage(request mqtt.Message) error
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

	GetSubscribedTopics() map[string]*SubscribeSet

	AppendSubscribedTopic(string, *SubscribeSet)

	RemoveSubscribedTopic(string)

	SetKeepaliveInterval(int)

	GetId() string

	GetRealId() string

	SetId(string)

	DisableCleanSession()

	ShouldCleanSession() bool

	IsBridge() bool
}
