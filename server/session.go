package server

import (
	codec "github.com/chobie/momonga/encoding/mqtt"
)

// ケッコー面倒くさい
type Session struct {
	Alived bool
	Connection Connection
	Queue []codec.Message
}

type SessionStore struct {
	Sessions map[string]*Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		Sessions: make(map[string]*Session),
	}
}

func (self *SessionStore) Attach(conn Connection) {
}

func (self *SessionStore) Detach(conn Connection) {
}
