// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
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
