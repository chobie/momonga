// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package datastore

import (
	"bytes"
	"errors"
	"github.com/chobie/momonga/skiplist"
	"sync"
)

type MemstoreIterator struct {
	Iterator *skiplist.SkipListIterator
}

func (self *MemstoreIterator) Seek(key []byte) {
	self.Iterator.Seek(key)
}

func (self *MemstoreIterator) Key() []byte {
	r := self.Iterator.Key()
	if v, ok := r.([]byte); ok {
		return v
	}
	return nil
}

func (self *MemstoreIterator) Value() []byte {
	r := self.Iterator.Value()
	if v, ok := r.([]byte); ok {
		return v
	}
	return nil
}

func (self *MemstoreIterator) Next() {
	self.Iterator.Next()
}

func (self *MemstoreIterator) Prev() {
	panic("prev is not supported yet")
}

func (self *MemstoreIterator) Valid() bool {
	return self.Iterator.Valid()
}

func (self *MemstoreIterator) Error() error {
	return nil
}

func (self *MemstoreIterator) Close() error {
	self.Iterator = nil
	return nil
}

func NewMemstore() *Memstore {
	return &Memstore{
		Storage: skiplist.NewSkipList(&skiplist.BytesComparator{}),
		Mutex:   &sync.RWMutex{},
	}
}

type Memstore struct {
	Storage *skiplist.SkipList
	Mutex   *sync.RWMutex
}

func (self *Memstore) Name() string {
	return "memstore"
}

func (self *Memstore) Path() string {
	return "inmemory"
}

func (self *Memstore) Put(key, value []byte) error {
	self.Mutex.Lock()
	defer func() {
		self.Mutex.Unlock()
	}()

	self.Storage.Delete(key)
	self.Storage.Insert(key, value)
	return nil
}

func (self *Memstore) Get(key []byte) ([]byte, error) {
	self.Mutex.RLock()
	defer func() {
		self.Mutex.RUnlock()
	}()

	itr := self.Iterator()
	itr.Seek(key)
	if itr.Valid() {
		return itr.Value(), nil
	}

	return nil, errors.New("not found")
}

func (self *Memstore) Del(first, last []byte) error {
	self.Mutex.RLock()
	itr := self.Iterator()

	var targets [][]byte
	for itr.Seek(first); itr.Valid(); itr.Next() {
		key := itr.Key()
		// The result will be 0 if a==b, -1 if a < b, and +1 if a > b.
		if bytes.Compare(key, last) > 0 {
			break
		}
		targets = append(targets, key)
	}
	self.Mutex.RUnlock()

	// MEMO: this is more safely.
	self.Mutex.Lock()
	for i := range targets {
		self.Storage.Delete(targets[i])
	}
	self.Mutex.Unlock()

	return nil
}

func (self *Memstore) Iterator() Iterator {
	return &MemstoreIterator{
		Iterator: self.Storage.Iterator(),
	}
}

func (self *Memstore) Close() error {
	self.Storage = nil
	return nil
}
