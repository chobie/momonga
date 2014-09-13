// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package server

import (
	"bytes"
	"github.com/chobie/momonga/encoding/mqtt"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/util"
	"sync"
	"time"
)

// MQTT Multiplexer Connection
//
// TODO: 途中で死んだとき用のやつを追加する.もうちょい素敵な実装にしたい
// TODO: goroutine safeにする
//
// Multiplexer、というかなんだろ。Engineとの仲介でおいとくやつ。
// 下のコネクションとかは純粋に接続周りだけにしておきたいんだけどなー
//
type MmuxConnection struct {
	// Primary
	PrimaryConnection Connection
	OfflineQueue      []mqtt.Message
	Connections       map[string]Connection
	MaxOfflineQueue   int
	Identifier        string
	CleanSession      bool
	OutGoingTable     *util.MessageTable
	SubscribeMap      map[string]bool
	Created           time.Time
	Hash              uint32
	Mutex             *sync.RWMutex
	SubscribedTopics  map[string]*SubscribeSet
}

func NewMmuxConnection() *MmuxConnection {
	conn := &MmuxConnection{
		OutGoingTable:    util.NewMessageTable(),
		Connections:      map[string]Connection{},
		SubscribeMap:     map[string]bool{},
		MaxOfflineQueue:  1000,
		Created:          time.Now(),
		Identifier:       "",
		Mutex:            &sync.RWMutex{},
		SubscribedTopics: make(map[string]*SubscribeSet),
	}

	return conn
}

func (self *MmuxConnection) GetId() string {
	return self.Identifier
}

func (self *MmuxConnection) Attach(conn Connection) {
	self.Mutex.Lock()
	defer self.Mutex.Unlock()

	if len(self.Connections) == 0 {
		self.PrimaryConnection = conn
		self.CleanSession = conn.ShouldClearSession()

		if conn.ShouldClearSession() {
			self.OfflineQueue = self.OfflineQueue[:0]
			self.SubscribeMap = make(map[string]bool)
			self.SubscribedTopics = make(map[string]*SubscribeSet)
			self.Connections = make(map[string]Connection)

			// Should I remove remaining QoS1, QoS2 message at this time?
			self.OutGoingTable.Clean()
		} else {
			if len(self.OfflineQueue) > 0 {
				log.Info("Process Offline Queue: Playback: %d, %d", len(self.OfflineQueue), len(self.Connections))
				for i := 0; i < len(self.OfflineQueue); i++ {
					self.WriteMessageQueue(self.OfflineQueue[i])
				}
				self.OfflineQueue = self.OfflineQueue[:0]
			}
		}
	}

	self.Connections[conn.GetRealId()] = conn
}

func (self *MmuxConnection) GetRealId() string {
	return self.Identifier
}

func (self *MmuxConnection) Detach(conn Connection) {
	if self.PrimaryConnection == conn {
		self.PrimaryConnection = nil
	}

	delete(self.Connections, conn.GetRealId())
	if len(self.Connections) == 0 {
		self.PrimaryConnection = nil
	} else {
		for _, v := range self.Connections {
			self.PrimaryConnection = v
			break
		}
	}
}

func (self *MmuxConnection) WriteMessageQueue(request mqtt.Message) {
	if self.PrimaryConnection == nil {
		if request.GetType() == mqtt.PACKET_TYPE_PUBLISH {
			if c, ok := request.(*mqtt.PublishMessage); ok {
				// 配送されないと思うけど念のため
				if c.Retain > 0 {
					// Don't keep retain message
					return
				}

				self.OfflineQueue = append(self.OfflineQueue, request)
			}
		} else {
			self.OfflineQueue = append(self.OfflineQueue, request)
		}
		return
	}
	self.PrimaryConnection.WriteMessageQueue(request)
}

func (self *MmuxConnection) WriteMessageQueue2(msg []byte) {
	if self.PrimaryConnection == nil {
		// めんどくせ
		r, _ := mqtt.ParseMessage(bytes.NewReader(msg), 0)
		self.OfflineQueue = append(self.OfflineQueue, r)
		return
	}

	self.PrimaryConnection.WriteMessageQueue2(msg)
}

func (self *MmuxConnection) Close() error {
	if self.PrimaryConnection == nil {
		return nil
	}
	return self.PrimaryConnection.Close()
}
func (self *MmuxConnection) SetState(state State) {
	if self.PrimaryConnection == nil {
		return
	}
	self.PrimaryConnection.SetState(state)
}

func (self *MmuxConnection) GetState() State {
	if self.PrimaryConnection == nil {
		return STATE_DETACHED
	}

	return self.PrimaryConnection.GetState()
}

func (self *MmuxConnection) ResetState() {
	if self.PrimaryConnection == nil {
		return
	}

	self.PrimaryConnection.ResetState()
}

func (self *MmuxConnection) ReadMessage() (mqtt.Message, error) {
	if self.PrimaryConnection == nil {
		return nil, nil
	}

	return self.PrimaryConnection.ReadMessage()
}

//func (self *MmuxConnection) Write(reader *bytes.Reader) error {
//	if self.PrimaryConnection == nil {
//		return nil
//	}
//
//	return self.PrimaryConnection.Write(reader)
//}

func (self *MmuxConnection) IsAlived() bool {
	if self.PrimaryConnection == nil {
		return true
	}

	return self.PrimaryConnection.IsAlived()
}

func (self *MmuxConnection) SetWillMessage(msg mqtt.WillMessage) {
	if self.PrimaryConnection == nil {
		return
	}

	self.PrimaryConnection.SetWillMessage(msg)
}

func (self *MmuxConnection) GetWillMessage() *mqtt.WillMessage {
	if self.PrimaryConnection == nil {
		return nil
	}
	return self.PrimaryConnection.GetWillMessage()
}

func (self *MmuxConnection) HasWillMessage() bool {
	if self.PrimaryConnection == nil {
		return false
	}
	return self.PrimaryConnection.HasWillMessage()

}

func (self *MmuxConnection) GetOutGoingTable() *util.MessageTable {
	return self.OutGoingTable
	//	if self.PrimaryConnection == nil {
	//		return nil
	//	}
	//
	//	return self.PrimaryConnection.GetOutGoingTable()
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
	if self.PrimaryConnection == nil {
		return
	}

	self.PrimaryConnection.SetKeepaliveInterval(interval)
}

func (self *MmuxConnection) DisableClearSession() {
}

func (self *MmuxConnection) ShouldClearSession() bool {
	if self.PrimaryConnection == nil {
		return self.CleanSession
	}

	return self.PrimaryConnection.ShouldClearSession()
}

func (self *MmuxConnection) GetHash() uint32 {
	return self.Hash
}

func (self *MmuxConnection) SetId(id string) {
	self.Identifier = id
	self.Hash = util.MurmurHash([]byte(id))
}
