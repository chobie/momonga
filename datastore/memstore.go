package datastore

import (
	"errors"
	"github.com/chobie/momonga/skiplist"
	"bytes"
	"fmt"
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
	panic("prev is not supported")
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
		Storage: skiplist.NewSkipList(&skiplist.BytesComparator{
		}),
	}
}

type Memstore struct {
	Storage *skiplist.SkipList
}

func (self *Memstore) Name() string {
	return "memstore"
}

func (self *Memstore) Path() string {
	return ""
}

func (self *Memstore) Put(key, value []byte) error {
	self.Storage.Insert(key, value)
	return nil
}

func (self *Memstore) Get(key []byte) ([]byte, error) {
	itr := self.Iterator()
	itr.Seek(key)

	if itr.Valid() {
		return itr.Value(), nil
	} else {
		fmt.Printf("damepo")
	}

	return nil, errors.New("not found")
}

func (self *Memstore) Del(first, last []byte) error {
	itr := self.Iterator()

	for itr.Seek(first); itr.Valid(); itr.Next() {
		key := itr.Key()
		if bytes.Compare(key, last) > 0{
			break
		}

		self.Storage.Delete(key)
	}

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
