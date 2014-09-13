// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package util

// TODO: goroutine safe
import (
	"errors"
	codec "github.com/chobie/momonga/encoding/mqtt"
	"sync"
	"time"
)

type MessageContainer struct {
	Message  codec.Message
	Refcount int
	Created  time.Time
	Updated  time.Time
	Opaque   interface{}
}

type MessageTable struct {
	sync.RWMutex
	Id       uint16
	Hash     map[uint16]*MessageContainer
	OnFinish func(uint16, codec.Message, interface{})
	used     map[uint16]bool
}

func NewMessageTable() *MessageTable {
	return &MessageTable{
		Id:   1,
		Hash: make(map[uint16]*MessageContainer),
		used: make(map[uint16]bool),
	}
}

func (self *MessageTable) SetOnFinish(callback func(uint16, codec.Message, interface{})) {
	self.OnFinish = callback
}

func (self *MessageTable) NewId() uint16 {
	self.Lock()
	if self.Id == 65535 {
		self.Id = 0
	}

	var id uint16
	ok := false
	for !ok {
		if _, ok = self.used[self.Id]; !ok {
			id = self.Id
			self.used[self.Id] = true
			self.Id++
			break
		}
	}

	self.Unlock()
	return id
}

func (self *MessageTable) Clean() {
	self.Lock()
	self.Hash = make(map[uint16]*MessageContainer)
	self.Unlock()
}

func (self *MessageTable) Get(id uint16) (codec.Message, error) {
	self.RLock()
	if v, ok := self.Hash[id]; ok {
		self.RUnlock()
		return v.Message, nil
	}

	self.RUnlock()
	return nil, errors.New("not found")
}

func (self *MessageTable) Register(id uint16, message codec.Message, opaque interface{}) {
	self.Lock()
	self.Hash[id] = &MessageContainer{
		Message:  message,
		Refcount: 1,
		Created:  time.Now(),
		Updated:  time.Now(),
		Opaque:   opaque,
	}
	self.Unlock()
}

func (self *MessageTable) Register2(id uint16, message codec.Message, count int, opaque interface{}) {
	self.Lock()
	self.Hash[id] = &MessageContainer{
		Message:  message,
		Refcount: count,
		Created:  time.Now(),
		Updated:  time.Now(),
		Opaque:   opaque,
	}
	self.Unlock()
}

func (self *MessageTable) Unref(id uint16) {
	self.Lock()
	if v, ok := self.Hash[id]; ok {
		v.Refcount--

		if v.Refcount < 1 {
			if self.OnFinish != nil {
				self.OnFinish(id, self.Hash[id].Message, self.Hash[id].Opaque)
			}
			delete(self.used, id)
			delete(self.Hash, id)
		}
	}
	self.Unlock()
}

func (self *MessageTable) Remove(id uint16) {
	self.Lock()
	if _, ok := self.Hash[id]; ok {
		delete(self.Hash, id)
	}
	self.Unlock()
}
