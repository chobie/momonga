package server

import (
	"net"
	"os"
)

type Listener interface {
	Accept() (c net.Conn, err error)

	Close() error

	Addr() net.Addr

	File() (f *os.File, err error)
}
