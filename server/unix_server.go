// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package server

import (
	"fmt"
	. "github.com/chobie/momonga/common"
	"github.com/chobie/momonga/configuration"
	log "github.com/chobie/momonga/logger"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type UnixServer struct {
	Engine   *Momonga
	Address  string
	stop     chan bool
	listener Listener
	inherit  bool
	wg       *sync.WaitGroup
	once     sync.Once
}

func NewUnixServer(engine *Momonga, config *configuration.Config, inherit bool) *UnixServer {
	t := &UnixServer{
		Engine:  engine,
		Address: config.GetSocketAddress(),
		stop:    make(chan bool, 1),
		inherit: inherit,
	}

	return t
}

func (self *UnixServer) ListenAndServe() error {
	if self.inherit {
		file := os.NewFile(uintptr(4), "sock")
		tmp, err := net.FileListener(file)
		file.Close()
		if err != nil {
			log.Error("UnixServer: %s", err)
			return nil
		}

		listener := tmp.(*net.TCPListener)
		self.listener = &MyListener{Listener: listener}
	} else {
		listener, err := net.Listen("unix", self.Address)

		if err != nil {
			panic(fmt.Sprintf("Error: %s", err))
			return err
		}
		self.listener = &MyListener{Listener: listener}
	}
	go self.Serve(self.listener)
	return nil
}

func (self *UnixServer) Serve(l net.Listener) error {
	log.Info("momonga_unix: started server")
	defer func() {
		l.Close()
	}()

	var tempDelay time.Duration // how long to sleep on accept failure

Accept:
	for {
		select {
		case <-self.stop:
			break Accept
		default:
			client, err := l.Accept()
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					if tempDelay == 0 {
						tempDelay = 5 * time.Millisecond
					} else {
						tempDelay *= 2
					}

					if max := 1 * time.Second; tempDelay > max {
						tempDelay = max
					}

					log.Info("momonga: Accept error: %v; retrying in %v", err, tempDelay)
					time.Sleep(tempDelay)
					continue
				}
				if strings.Contains(err.Error(), "use of closed network connection") {
					log.Error("Accept Failed: %s", err)
					continue
				}

				return err
			}
			tempDelay = 0

			conn := NewMyConnection(nil)
			conn.SetMyConnection(client)
			conn.SetId(client.RemoteAddr().String())

			log.Debug("Accepted: %s", conn.GetId())
			go self.Engine.HandleConnection(conn)
		}
	}

	self.listener.(*MyListener).wg.Wait()
	self.once.Do(func() {
		self.wg.Done()
	})
	return nil
}

func (self *UnixServer) Stop() {
	close(self.stop)
	self.listener.Close()
}

func (self *UnixServer) Graceful() {
	log.Info("stop new accepting")
	close(self.stop)
	self.listener.Close()
}

func (self *UnixServer) Listener() Listener {
	return self.listener
}
