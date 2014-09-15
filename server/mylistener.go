package server

import (
	log "github.com/chobie/momonga/logger"
	"net"
	"os"
	"sync"
)

type MyListener struct {
	net.Listener
	mutex sync.RWMutex
	wg    sync.WaitGroup
	close chan bool
}

func (self *MyListener) Accept() (net.Conn, error) {
	var c net.Conn

	c, err := self.Listener.Accept()
	if err != nil {
		return nil, err
	}
	defer func() {
		// If we didn't accept, we decrement our presumptuous count above.
		if c == nil {
			self.wg.Done()
		}
	}()

	self.wg.Add(1)
	return &MyConn{Conn: c, wg: &self.wg}, nil
}

func (self *MyListener) File() (f *os.File, err error) {
	if tl, ok := self.Listener.(*net.TCPListener); ok {
		file, _ := tl.File()
		return file, nil
	}
	log.Info("MyConn Failed to convert file: %T", self.Listener)

	return nil, nil
}

type MyConn struct {
	net.Conn
	wg   *sync.WaitGroup
	once sync.Once
}

func (self *MyConn) Close() error {
	err := self.Conn.Close()
	self.once.Do(func() {
		self.wg.Done()
	})
	return err
}
