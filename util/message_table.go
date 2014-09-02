package util

import (
	codec "github.com/chobie/momonga/encoding/mqtt"
	"time"
	"errors"
)

type MessageContainer struct {
	Message codec.Message
	Refcount int
	Created time.Time
	Updated time.Time
	Opaque interface{}
}

type MessageTable struct {
	Id uint16
	Hash map[uint16]*MessageContainer
	OnFinish func(uint16, codec.Message, interface{})
}

func NewMessageTable() *MessageTable {
	return &MessageTable{
		Id: 0,
		Hash: make(map[uint16]*MessageContainer),
	}
}

func (self *MessageTable) SetOnFinish(callback func(uint16, codec.Message, interface{})) {
	self.OnFinish = callback
}

func (self *MessageTable) NewId() uint16 {
	if self.Id == 65535 {
		self.Id = 0
	}
	self.Id++

	return self.Id
}

func (self *MessageTable) Get(id uint16) (codec.Message, error) {
	if v, ok := self.Hash[id]; ok {
		return v.Message, nil
	}
	return nil, errors.New("not found")
}


func (self *MessageTable) Register(id uint16, message codec.Message, opaque interface{}) {
	self.Hash[id] = &MessageContainer{
		Message: message,
		Refcount: 0,
		Created: time.Now(),
		Updated: time.Now(),
		Opaque: opaque,
	}
}

func (self *MessageTable) Register2(id uint16, message codec.Message, count int, opaque interface{}) {
	self.Hash[id] = &MessageContainer{
		Message: message,
		Refcount: count,
		Created: time.Now(),
		Updated: time.Now(),
		Opaque: opaque,
	}
}

func (self *MessageTable) Unref(id uint16) {
	if v, ok := self.Hash[id]; ok {
		v.Refcount--

		if v.Refcount < 1 {
			if self.OnFinish != nil {
				self.OnFinish(id, self.Hash[id].Message, self.Hash[id].Opaque)
			}
			delete(self.Hash, id)
		}
	}
}

func (self *MessageTable) Remove(id uint16) {
	if _, ok := self.Hash[id]; ok {
		delete(self.Hash, id)
	}
}
