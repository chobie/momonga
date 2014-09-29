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
	"crypto/tls"
)

type TlsServer struct {
	ListenAddress string
	Engine        *Momonga
	config        *configuration.Config
	stop          chan bool
	listener      Listener
	inherit       bool
	wg            *sync.WaitGroup
	once          sync.Once
	tlsConfig      *tls.Config
}

func NewTlsServer(engine *Momonga, config *configuration.Config, inherit bool) *TlsServer {
	t := &TlsServer{
		Engine:        engine,
		ListenAddress: config.GetTlsListenAddress(),
		config:        config,
		stop:          make(chan bool, 1),
		inherit:       inherit,
	}

	return t
}

func (self *TlsServer) ListenAndServe() error {
	if self.inherit {
		file := os.NewFile(uintptr(3), "sock")
		tmp, err := net.FileListener(file)
		file.Close()
		if err != nil {
			log.Error("Error: %s", err)
			return nil
		}

		cert, err := tls.LoadX509KeyPair(self.config.Server.Certfile, self.config.Server.Keyfile)
		if err != nil {
			panic(fmt.Sprintf("LoadX509KeyPair error: %s", err))
		}

		config := &tls.Config{Certificates: []tls.Certificate{cert}}
		self.tlsConfig = config
		// TODO: IS THIS CORRECT?
		listener := tmp.(net.Listener)
		self.listener = &MyListener{Listener: listener}
	} else {
		cert, err := tls.LoadX509KeyPair(self.config.Server.Certfile, self.config.Server.Keyfile)
		if err != nil {
			panic(fmt.Sprintf("LoadX509KeyPair error: %s", err))
		}

		config := &tls.Config{Certificates: []tls.Certificate{cert}}
		listener, err := tls.Listen("tcp", self.ListenAddress, config)
		self.tlsConfig = config

		if err != nil {
			panic(fmt.Sprintf("Error: %s", err))
		}

		self.listener = &MyListener{Listener: listener}
	}

	log.Info("momonga_tls: started tls server: %s", self.listener.Addr().String())
	for i := 0; i < self.config.GetAcceptorCount(); i++ {
		go self.Serve(self.listener)
	}

	return nil
}

func (self *TlsServer) Serve(l net.Listener) error {
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

func (self *TlsServer) Stop() {
	close(self.stop)
	self.listener.Close()
}

func (self *TlsServer) Graceful() {
	log.Info("stop new accepting")
	close(self.stop)
	self.listener.Close()

}

func (self *TlsServer) Listener() Listener {
	log.Info("LIS: %#v\n", self.listener)
	return self.listener
}
