// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package datastore

type Iterator interface {
	Seek(key []byte)
	Key() []byte
	Value() []byte
	Next()
	Prev()
	Valid() bool
	Error() error
	Close() error
}

type Datastore interface {
	Name() string
	Path() string
	Put(key, value []byte) error
	Get(key []byte) ([]byte, error)
	Del(first, last []byte) error
	Iterator() Iterator
	Close() error
}
