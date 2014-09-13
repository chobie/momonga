package datastore

import (
	_ "fmt"
	. "gopkg.in/check.v1"
	"testing"
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

	memstore.Del([]byte("key1"), []byte("key1"))
	//TopicA/C
	//TopicA/B

	memstore = NewMemstore()
	memstore.Put([]byte("$SYS/broker/broker/version"), []byte("a"))
	memstore.Put([]byte("Topic/C"), []byte("a"))
	memstore.Put([]byte("TopicA/C"), []byte("a"))
	memstore.Put([]byte("TopicA/B"), []byte("a"))

	itr := memstore.Iterator()
	var targets []string
	for ; itr.Valid(); itr.Next() {
		x := itr.Key()
		targets = append(targets, string(x))
	}
	for _, s := range targets {
		memstore.Del([]byte(s), []byte(s))
	}

	_, err = memstore.Get([]byte("Topic/C"))
	c.Assert(err.Error(), Equals, "not found")

	//	fmt.Printf("\n")
	//	for itr := memstore.Iterator(); itr.Valid(); itr.Next() {
	//		fmt.Printf("key: %s\n", itr.Key())
	//	}

	//
	//	value, err = memstore.Get([]byte("key_nothing"))
	//	c.Assert(err.Error(), Equals, "not found")
	//	c.Assert(value, DeepEquals, []byte(nil))
	//
}
