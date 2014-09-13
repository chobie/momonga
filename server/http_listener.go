package server

import (
	"net"
	"os"
)

type HttpListener struct {
	net.Listener
	WebSocketMount string
}

func NewHttpListener(listener net.Listener) *HttpListener {
	l := &HttpListener{
		Listener: listener,
	}

	return l
}

func (self *HttpListener) Accept() (c net.Conn, err error) {
	return self.Listener.Accept()
}

func (self *HttpListener) Close() error {
	return self.Listener.Close()
}

func (self *HttpListener) Addr() net.Addr {
	return self.Listener.Addr()
}

func (self *HttpListener) File() (f *os.File, err error) {
	return nil, nil
}
