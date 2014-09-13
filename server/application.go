// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package server

import (
	"sync"
)

type Application struct {
	Engine *Momonga
	Servers []Server
	wg sync.WaitGroup
}

func NewApplication(engine *Momonga) *Application{
	app := &Application{
		Engine: engine,
		Servers: make([]Server, 0),
	}

	return app
}

func (self *Application) Start() {
	self.wg.Add(1)
	go self.Engine.Run()

	for i := 0; i < len(self.Servers); i++ {
		svr := self.Servers[i]
		go svr.ListenAndServe()
		self.wg.Add(1)
	}
}

func (self *Application) Loop() {
	self.wg.Wait()
}

func (self *Application) RegisterServer(svr Server) {
	self.Servers = append(self.Servers, svr)
}
