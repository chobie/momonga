// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package server

import (
	log "github.com/chobie/momonga/logger"
	"net"
	"os"
	"sync"
)

type HttpListener struct {
	net.Listener
	WebSocketMount string
	mutex          sync.RWMutex
	wg             sync.WaitGroup
	close          chan bool
}

func NewHttpListener(listener net.Listener) *HttpListener {
	l := &HttpListener{
		Listener: listener,
	}

	return l
}

func (self *HttpListener) Accept() (c net.Conn, err error) {
	c, err = self.Listener.Accept()
	if err != nil {
		return nil, err
	}

	self.wg.Add(1)
	return &MyConn{Conn: c, wg: &self.wg}, nil
}

func (self *HttpListener) Close() error {
	return self.Listener.Close()
}

func (self *HttpListener) Addr() net.Addr {
	return self.Listener.Addr()
}

func (self *HttpListener) File() (f *os.File, err error) {
	if tl, ok := self.Listener.(*net.TCPListener); ok {
		file, _ := tl.File()
		return file, nil
	}
	log.Info("HttpListener Failed to convert file: %T", self.Listener)

	return nil, nil
}
