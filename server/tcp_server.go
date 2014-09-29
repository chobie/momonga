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

type TcpServer struct {
	ListenAddress string
	Engine        *Momonga
	config        *configuration.Config
	stop          chan bool
	listener      Listener
	inherit       bool
	wg            *sync.WaitGroup
	once          sync.Once
}

func NewTcpServer(engine *Momonga, config *configuration.Config, inherit bool) *TcpServer {
	t := &TcpServer{
		Engine:        engine,
		ListenAddress: config.GetListenAddress(),
		config:        config,
		stop:          make(chan bool, 1),
		inherit:       inherit,
	}

	return t
}

func (self *TcpServer) ListenAndServe() error {
	if self.inherit {
		file := os.NewFile(uintptr(3), "sock")
		tmp, err := net.FileListener(file)
		file.Close()
		if err != nil {
			log.Error("Error: %s", err)
			return nil
		}

		listener := tmp.(*net.TCPListener)
		self.listener = &MyListener{Listener: listener}
	} else {
		addr, err := net.ResolveTCPAddr("tcp4", self.ListenAddress)
		listener, err := net.ListenTCP("tcp", addr)

		if err != nil {
			panic(fmt.Sprintf("Error: %s", err))
			return err
		}

		self.listener = &MyListener{Listener: listener}
	}

	log.Info("momonga_tcp: started tcp server: %s", self.listener.Addr().String())
	for i := 0; i < self.config.GetAcceptorCount(); i++ {
		go self.Serve(self.listener)
	}

	return nil
}

func (self *TcpServer) Serve(l net.Listener) error {
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
				if v, ok := client.(*net.TCPConn); ok {
					v.SetNoDelay(true)
					v.SetKeepAlive(true)
				}

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

				log.Error("Accept Error: %s", err)
				return err
			}
			tempDelay = 0


			myconf := GetDefaultMyConfig()
			myconf.MaxMessageSize = self.Engine.Config().Server.MessageSizeLimit
			conn := NewMyConnection(myconf)
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

func (self *TcpServer) Stop() {
	close(self.stop)
	self.listener.Close()
}

func (self *TcpServer) Graceful() {
	log.Info("stop new accepting")
	close(self.stop)
	self.listener.Close()

}

func (self *TcpServer) Listener() Listener {
	log.Info("LIS: %#v\n", self.listener)
	return self.listener
}
