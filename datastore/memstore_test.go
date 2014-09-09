package datastore

import (
	"testing"
	. "gopkg.in/check.v1"
	_ "fmt"
)

func Test(t *testing.T) { TestingT(t) }

type DatastoreSuite struct{}

var _ = Suite(&DatastoreSuite{})

func (s *DatastoreSuite) TestMemstore(c *C) {
	memstore := NewMemstore()
	var err error

	err = memstore.Put([]byte("key1"), []byte("value1"))
	c.Assert(err, Equals, nil)
	err = memstore.Put([]byte("key2"), []byte("value2"))
	c.Assert(err, Equals, nil)
	err = memstore.Put([]byte("key3"), []byte("value3"))
	c.Assert(err, Equals, nil)

	value, err := memstore.Get([]byte("key1"))
	c.Assert(err, Equals, nil)
	c.Assert(value, DeepEquals, []byte("value1"))
//
//	value, err = memstore.Get([]byte("key_nothing"))
//	c.Assert(err.Error(), Equals, "not found")
//	c.Assert(value, DeepEquals, []byte(nil))
//
}

