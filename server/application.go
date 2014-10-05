// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
package server

import (
	codec "github.com/chobie/momonga/encoding/mqtt"
	"github.com/chobie/momonga/client"
	"github.com/chobie/momonga/configuration"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/util"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"sync"
	"syscall"
	"time"
	"fmt"
	"net"
)

//
// Application manages start / stop, singnal handling and listeners.
//
// +-----------+
// |APPLICATION| start / stop, signal handling
// +-----------+
// |  LISTENER | listen and accept
// +-----------+
// |  HANDLER  | parse and execute commands
// +-----------+
// |  ENGINE   | implements commands api
// +-----------+
//
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

	// NOTE: INHERIT=TRUE means the process invoked from os.StartProess
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

	// TODO: improve this block
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

	if conf.Server.EnableTls && conf.Server.TlsPort > 0 {
		h := NewTlsServer(engine, conf, inherit)
		h.wg = &app.wg
		app.RegisterServer(h)
	}

	// memonized application path
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

	if conf.Bridge.Address != "" {
		//TODO: Bridgeは複数指定できる
		//TODO: Bridgeは途中で制御できる
		// /api/bridge/list
		// /api/bridge/connection/stop
		// /api/bridge/connection/status
		// /api/bridge/connection/start
		// /api/bridge/connection/delete
		// /api/bridge/connection/new?address=&port=&type=both&topic[]=
		// /api/bridge/config
		go func() {
			flag := 0
			switch conf.Bridge.Type {
			case "both":
				flag = 3
			case "out":
				flag = 1
			case "in":
				flag = 2
			default:
				panic(fmt.Sprintf("%s does not support.", conf.Bridge.Type))
			}

			// in
			addr := fmt.Sprintf("%s:%d", conf.Bridge.Address, conf.Bridge.Port)
			c := client.NewClient(client.Option{
				TransporterCallback: func() (net.Conn, error) {
					conn, err := net.Dial("tcp", addr)
					return conn, err
				},
				Identifier: fmt.Sprintf(conf.Bridge.ClientId),
				Magic:      []byte("MQTT"),
				Version:    4 | 0x80,
				Keepalive:  0,
			})

			c.Connect()
			c.WaitConnection()
			if flag == 1 || flag == 3 {
				c.Subscribe("#", 2)
				c.SetRequestPerSecondLimit(-1)
				c.On("publish", func(msg *codec.PublishMessage) {
						engine.SendPublishMessage(msg, conf.Bridge.Connection, true)
				})
			}

			//out
			if flag == 2 || flag == 3 {
				addr2 := fmt.Sprintf("%s:%d", conf.Server.BindAddress, conf.Server.Port)
				c2 := client.NewClient(client.Option{
					TransporterCallback: func() (net.Conn, error) {
						conn, err := net.Dial("tcp", addr2)
						return conn, err
					},
					Identifier: fmt.Sprintf(conf.Bridge.ClientId),
					Magic:      []byte("MQTT"),
					Version:    4 | 0x80,
					Keepalive:  0,
				})

				c2.Connect()
				c2.WaitConnection()
				c2.Subscribe("#", 2)
				c2.SetRequestPerSecondLimit(-1)
				c2.On("publish", func(msg *codec.PublishMessage) {
						c.Publish(msg.TopicName, msg.Payload, msg.QosLevel)
					})
			}
			select{}
		}()
	}

	return app
}

func (self *Application) Start() {
	self.wg.Add(1)

	ch := make(chan os.Signal, 8)
	// TODO: windows can't use signal. split this block into another file.
	signals := []os.Signal{syscall.SIGINT, syscall.SIGHUP, syscall.SIGUSR2, syscall.SIGQUIT}
	signal.Notify(ch, signals...)

	go func(ch chan os.Signal) {
		for {
			select {
			case x := <-ch:
				switch x {
				case syscall.SIGINT:
					self.Stop()
				case syscall.SIGQUIT:
					// TODO: like sigdump feature. should change file descriptor
					pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
					pprof.Lookup("heap").WriteTo(os.Stdout, 1)
					pprof.Lookup("block").WriteTo(os.Stdout, 1)

				case syscall.SIGHUP:
					// reload config
					log.Info("reload configuration from %s", self.configPath)
					configuration.LoadConfigurationTo(self.configPath, self.config)

				case syscall.SIGUSR2:
					// graceful restart
					self.mu.Lock()
					var env []string
					for _, v := range os.Environ() {
						env = append(env, v)
					}

					discriptors := append([]*os.File{
						os.Stdin,
						os.Stdout,
						os.Stderr,
					})

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

					// Kill current connection in N seconds.
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
	self.Engine.Terminate()

	self.wg.Done()
}

func (self *Application) Loop() {
	self.wg.Wait()
}

func (self *Application) RegisterServer(svr Server) {
	self.Servers = append(self.Servers, svr)
}
