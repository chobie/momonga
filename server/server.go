// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package server

import (
	"net"
)

type Server interface {
	ListenAndServe() error
	//ListenAndServeTLS(certFile, keyFile string) error
	Serve(l net.Listener) error

	Graceful()

	Stop()

	Listener() Listener

	//	Restart()
}
