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
