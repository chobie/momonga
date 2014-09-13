package server

import (
	"net"
)

type Server interface{
	ListenAndServe() error
	//ListenAndServeTLS(certFile, keyFile string) error
	Serve(l net.Listener) error
//	Graceful()
//	Stop()
//	Restart()
}
