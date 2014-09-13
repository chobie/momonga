// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package server

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Application struct {
	Engine  *Momonga
	Servers []Server
	wg      sync.WaitGroup
}

func NewApplication(engine *Momonga) *Application {
	app := &Application{
		Engine:  engine,
		Servers: make([]Server, 0),
	}

	return app
}

func (self *Application) Start() {
	self.wg.Add(1)

	ch := make(chan os.Signal, 1)
	signals := []os.Signal{syscall.SIGINT}
	signal.Notify(ch, signals...)
	go func(ch chan os.Signal) {
		for {
			select {
			case x := <-ch:
				switch x {
				case syscall.SIGINT:
					self.Stop()
				}
			}
		}
	}(ch)

	go self.Engine.Run()
	for i := 0; i < len(self.Servers); i++ {
		svr := self.Servers[i]
		go svr.ListenAndServe()
		self.wg.Add(1)
	}
}

func (self *Application) Stop() {
	for i := 0; i < len(self.Servers); i++ {
		svr := self.Servers[i]
		svr.Stop()
		self.wg.Done()
	}

	self.wg.Done()
}

func (self *Application) Loop() {
	self.wg.Wait()
}

func (self *Application) RegisterServer(svr Server) {
	self.Servers = append(self.Servers, svr)
}
