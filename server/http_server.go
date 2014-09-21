// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package server

import (
	"fmt"
	"github.com/chobie/momonga/configuration"
	log "github.com/chobie/momonga/logger"
	"net"
	"net/http"
	"os"
	"sync"
)

type HttpServer struct {
	http.Server
	Engine   *Momonga
	Address  string
	stop     chan bool
	listener Listener
	inherit  bool
	once     sync.Once
	wg       *sync.WaitGroup
}

func NewHttpServer(engine *Momonga, config *configuration.Config, inherit bool) *HttpServer {
	t := &HttpServer{
		Server: http.Server{
			Handler: &MyHttpServer{
				Engine:         engine,
				WebSocketMount: config.Server.WebSocketMount,
			},
		},
		Engine:  engine,
		Address: fmt.Sprintf(":%d", config.Server.HttpPort),
		stop:    make(chan bool, 1),
		inherit: inherit,
	}

	return t
}

func (self *HttpServer) ListenAndServe() error {
	if self.inherit {
		file := os.NewFile(uintptr(5), "sock")
		tmp, err := net.FileListener(file)
		file.Close()
		if err != nil {
			log.Error("HttpServer: %s", err)
			return nil
		}
		listener := tmp.(*net.TCPListener)
		self.listener = NewHttpListener(listener)
	} else {
		addr, err := net.ResolveTCPAddr("tcp4", self.Address)
		base, err := net.ListenTCP("tcp", addr)

		listener := NewHttpListener(base)
		if err != nil {
			return err
		}

		self.listener = listener
	}
	return self.Serve(self.listener)
}

func (self *HttpServer) Serve(l net.Listener) error {
	log.Info("momonga_http: started http server: %s", l.Addr().String())

	// アッー. Stop出来ねぇ
	go self.Server.Serve(l)
	return nil
}

func (self *HttpServer) Stop() {
	close(self.stop)

	self.once.Do(func() {
		self.listener.(*HttpListener).wg.Wait()
		log.Info("Finished HTTP Listener")
		self.wg.Done()
	})
}

func (self *HttpServer) Graceful() {
	self.Stop()
}

func (self *HttpServer) Listener() Listener {
	return self.listener
}
