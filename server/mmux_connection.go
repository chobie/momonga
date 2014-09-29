// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package server

import (
	. "github.com/chobie/momonga/common"
	"github.com/chobie/momonga/encoding/mqtt"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/util"
	"sync"
	"time"
	"unsafe"
	"sync/atomic"
	"fmt"
)

// MQTT Multiplexer Connection
//
// TODO: 途中で死んだとき用のやつを追加する.もうちょい素敵な実装にしたい
//
// Multiplexer、というかなんだろ。EngineとConnectionとの仲介でおいとくやつ。
// Sessionがあるのでこういうふうにしとくと楽かな、と
type MmuxConnection struct {
	// Primary
	Connection *Connection
	OfflineQueue      []mqtt.Message
	MaxOfflineQueue   int
	Identifier        string
	CleanSession      bool
	OutGoingTable     *util.MessageTable
	SubscribeMap      map[string]bool
	Created           time.Time
	Hash              uint32
	Mutex             sync.RWMutex
	SubscribedTopics  map[string]*SubscribeSet
}

func NewMmuxConnection() *MmuxConnection {
	conn := &MmuxConnection{
		OutGoingTable:    util.NewMessageTable(),
		SubscribeMap:     map[string]bool{},
		MaxOfflineQueue:  1000,
		Created:          time.Now(),
		Identifier:       "",
		SubscribedTopics: make(map[string]*SubscribeSet),
		CleanSession:     true,
	}

	return conn
}

func (self *MmuxConnection) GetId() string {
	return self.Identifier
}

func (self *MmuxConnection) Attach(conn Connection) {
	self.Mutex.Lock()
	defer self.Mutex.Unlock()

	var container Connection
	container = conn
	old := atomic.SwapPointer((*unsafe.Pointer)((unsafe.Pointer)(&self.Connection)), unsafe.Pointer(&container))
	if old != nil {
		// 1.If the ClientId represents a Client already connected to the Server
		// then the Server MUST disconnect the existing Client [MQTT-3.1.4-2].
		(*(*Connection)(old)).Close()
		fmt.Printf("close existing connection")
	}

	self.CleanSession = conn.ShouldCleanSession()
	if conn.ShouldCleanSession() {
		self.OfflineQueue = self.OfflineQueue[:0]
		self.SubscribeMap = make(map[string]bool)
		self.SubscribedTopics = make(map[string]*SubscribeSet)

		// Should I remove remaining QoS1, QoS2 message at this time?
		self.OutGoingTable.Clean()
	} else {
		if len(self.OfflineQueue) > 0 {
			log.Info("Process Offline Queue: Playback: %d", len(self.OfflineQueue))
			for i := 0; i < len(self.OfflineQueue); i++ {
				self.writeMessageQueue(self.OfflineQueue[i])
			}
			self.OfflineQueue = self.OfflineQueue[:0]
		}
	}
}

func (self *MmuxConnection) GetRealId() string {
	return self.Identifier
}

func (self *MmuxConnection) Detach(conn Connection, dummy *DummyPlug) {
	var container Connection
	container = dummy

	atomic.SwapPointer((*unsafe.Pointer)((unsafe.Pointer)(&self.Connection)), unsafe.Pointer(&container))
}

func (self *MmuxConnection) WriteMessageQueue(request mqtt.Message) {
	self.writeMessageQueue(request)
}

// Without lock
func (self *MmuxConnection) writeMessageQueue(request mqtt.Message) {
	// TODO: これそもそもmuxがあったらもうdummyか普通のか、ぐらいなような気が
	_, is_dummy := (*self.Connection).(*DummyPlug)
	if self.Connection == nil {
		// already disconnected
		return
	} else if is_dummy {
		if request.GetType() == mqtt.PACKET_TYPE_PUBLISH {
			self.OfflineQueue = append(self.OfflineQueue, request)
		}
	}

	(*self.Connection).WriteMessageQueue(request)
}

func (self *MmuxConnection) Close() error {
	return (*self.Connection).Close()
}

func (self *MmuxConnection) SetState(state State) {
	(*self.Connection).SetState(state)
}

func (self *MmuxConnection) GetState() State {
	return (*self.Connection).GetState()
}

func (self *MmuxConnection) ResetState() {
	(*self.Connection).ResetState()
}

func (self *MmuxConnection) ReadMessage() (mqtt.Message, error) {
	return (*self.Connection).ReadMessage()
}

func (self *MmuxConnection) IsAlived() bool {
	return (*self.Connection).IsAlived()
}

func (self *MmuxConnection) SetWillMessage(msg mqtt.WillMessage) {
	self.Mutex.Lock()
	defer self.Mutex.Unlock()

	(*self.Connection).SetWillMessage(msg)
}

func (self *MmuxConnection) GetWillMessage() *mqtt.WillMessage {
	self.Mutex.RLock()
	defer self.Mutex.RUnlock()

	return (*self.Connection).GetWillMessage()
}

func (self *MmuxConnection) HasWillMessage() bool {
	self.Mutex.RLock()
	defer self.Mutex.RUnlock()

	return (*self.Connection).HasWillMessage()
}

func (self *MmuxConnection) GetOutGoingTable() *util.MessageTable {
	self.Mutex.RLock()
	defer self.Mutex.RUnlock()

	return self.OutGoingTable
	//	if self.Connection == nil {
	//		return nil
	//	}
	//
	//	return self.Connection.GetOutGoingTable()
}

func (self *MmuxConnection) GetSubscribedTopics() map[string]*SubscribeSet {
	self.Mutex.RLock()
	defer self.Mutex.RUnlock()

	return self.SubscribedTopics
}

func (self *MmuxConnection) AppendSubscribedTopic(topic string, set *SubscribeSet) {
	self.Mutex.Lock()
	defer self.Mutex.Unlock()

	self.SubscribedTopics[topic] = set
	self.SubscribeMap[topic] = true
}

func (self *MmuxConnection) IsSubscribed(topic string) bool {
	self.Mutex.RLock()
	defer self.Mutex.RUnlock()

	if _, ok := self.SubscribeMap[topic]; ok {
		return true
	}
	return false
}

func (self *MmuxConnection) RemoveSubscribedTopic(topic string) {
	self.Mutex.Lock()
	defer self.Mutex.Unlock()

	if _, ok := self.SubscribedTopics[topic]; ok {
		delete(self.SubscribedTopics, topic)
		delete(self.SubscribeMap, topic)
	}
}

func (self *MmuxConnection) SetKeepaliveInterval(interval int) {
	self.Mutex.Lock()
	defer self.Mutex.Unlock()

	(*self.Connection).SetKeepaliveInterval(interval)
}

func (self *MmuxConnection) DisableCleanSession() {
}

func (self *MmuxConnection) ShouldCleanSession() bool {
	return (*self.Connection).ShouldCleanSession()
}

func (self *MmuxConnection) GetHash() uint32 {
	return self.Hash
}

func (self *MmuxConnection) SetId(id string) {
	self.Mutex.Lock()
	defer self.Mutex.Unlock()

	self.Identifier = id
	self.Hash = util.MurmurHash([]byte(id))
}
