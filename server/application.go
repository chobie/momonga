// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package server

import (
	"fmt"
	"github.com/chobie/momonga/configuration"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/util"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type Application struct {
	Engine     *Momonga
	Servers    []Server
	wg         sync.WaitGroup
	configPath string
	config     *configuration.Config
	execPath   string
	workingDir string
	mu         sync.Mutex
}

func NewApplication(configPath string) *Application {
	conf, err := configuration.LoadConfiguration(configPath)
	if err != nil {
		log.Error("Can't read config.toml. use default setting.: %s", err)
	}
	log.SetupLogging(conf.Server.LogLevel, conf.Server.LogFile)
	pid := strconv.Itoa(os.Getpid())
	if conf.Server.PidFile != "" {
		if err := ioutil.WriteFile(conf.Server.PidFile, []byte(pid), 0644); err != nil {
			panic(err)
		}
		util.WritePid(conf.Server.PidFile)
	}

	inherit := false
	if os.Getenv("INHERIT") == "TRUE" {
		inherit = true
	}

	log.Info("Momonga started pid: %s (inherit:%t)", pid, inherit)
	engine := NewMomonga(conf)
	app := &Application{
		Engine:     engine,
		Servers:    []Server{},
		configPath: configPath,
		config:     conf,
	}

	if conf.Server.Port > 0 {
		t := NewTcpServer(engine, conf, inherit)
		t.wg = &app.wg
		app.RegisterServer(t)
	}
	if conf.Server.Socket != "" {
		u := NewUnixServer(engine, conf, inherit)
		u.wg = &app.wg
		app.RegisterServer(u)
	}
	if conf.Server.HttpPort > 0 {
		h := NewHttpServer(engine, conf, inherit)
		h.wg = &app.wg
		app.RegisterServer(h)
	}

	app.execPath, err = exec.LookPath(os.Args[0])
	if err != nil {
		log.Error("Error: %s", err)
		return app
	}
	app.workingDir, err = os.Getwd()
	if err != nil {
		log.Error("Error: %s", err)
		return app
	}

	return app
}

func (self *Application) Start() {
	self.wg.Add(1)

	ch := make(chan os.Signal, 1)
	signals := []os.Signal{syscall.SIGINT, syscall.SIGHUP, syscall.SIGUSR2}
	signal.Notify(ch, signals...)

	go func(ch chan os.Signal) {
		for {
			select {
			case x := <-ch:
				switch x {
				case syscall.SIGINT:
					self.Stop()
				case syscall.SIGHUP:
					// reload config
					log.Info("reload configuration from %s", self.configPath)
					configuration.LoadConfigurationTo(self.configPath, self.config)
				case syscall.SIGUSR2:
					self.mu.Lock()
					// graceful restart
					var env []string
					for _, v := range os.Environ() {
						env = append(env, v)
					}

					discriptors := append([]*os.File{
						os.Stdin,
						os.Stdout,
						os.Stderr,
					})

					fmt.Printf("srv: %#v", self.Servers)
					for i := 0; i < len(self.Servers); i++ {
						svr := self.Servers[i]
						f, e := svr.Listener().File()

						if e != nil {
							log.Error("Error: %s", e)
							self.mu.Unlock()
							continue
						}

						fd, _ := syscall.Dup(int(f.Fd()))
						fx := os.NewFile(uintptr(fd), "sock")
						discriptors = append(discriptors, []*os.File{fx}...)
					}

					env = append(env, "INHERIT=TRUE")
					p, err := os.StartProcess(self.execPath, os.Args, &os.ProcAttr{
						Dir:   self.workingDir,
						Env:   env,
						Files: discriptors,
					})

					if err != nil {
						log.Error("Error: %s, stop gracefull restart.", err)
						p.Kill()
						self.mu.Unlock()
						continue
					}

					// maybe, new server is alive in 3 seconds
					time.Sleep(time.Second * 3)
					for i := 0; i < len(self.Servers); i++ {
						svr := self.Servers[i]
						svr.Graceful()
					}
					self.wg.Done()
					self.Engine.Doom()

					self.mu.Unlock()
					return
				}
			}
		}
	}(ch)

	go self.Engine.Run()
	for i := 0; i < len(self.Servers); i++ {
		svr := self.Servers[i]
		self.wg.Add(1)
		go svr.ListenAndServe()
	}
}

func (self *Application) Stop() {
	for i := 0; i < len(self.Servers); i++ {
		svr := self.Servers[i]
		svr.Stop()
	}

	self.wg.Done()
}

func (self *Application) Loop() {
	self.wg.Wait()
}

func (self *Application) RegisterServer(svr Server) {
	self.Servers = append(self.Servers, svr)
}
