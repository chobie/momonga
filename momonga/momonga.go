// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package main

import (
	"flag"
	"github.com/chobie/momonga/configuration"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/server"
	"github.com/chobie/momonga/util"
	"io/ioutil"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
)

func setupEngine(engine *server.Momonga) {
	//これハメだよなー
	engine.SetupCallback()
}

func main() {
	foreGround := flag.Bool("foreground", true, "run as foreground")
	configFile := flag.String("config", "config.toml", "the config file")
	pidFile := flag.String("pidfile", "", "the pid file")
	flag.Parse()

	f, _ := os.Create("profiler")
	pprof.StartCPUProfile(f)
	defer func() {
		pprof.StopCPUProfile()
		os.Exit(0)
	}()

	conf, err := configuration.LoadConfiguration(*configFile)
	if err != nil {
		log.Error("Can't read config.toml. use default setting.: %s", err)
	}
	if *pidFile != "" {
		conf.Server.PidFile = *pidFile
	}

	log.SetupLogging(conf.Server.LogLevel, conf.Server.LogFile)
	if !*foreGround {
		err := util.Daemonize(0, 0)
		if err != 0 {
			log.Info("fork failed")
			os.Exit(-1)
		}
	}

	if conf.Server.PidFile != "" {
		pid := strconv.Itoa(os.Getpid())
		if err := ioutil.WriteFile(conf.Server.PidFile, []byte(pid), 0644); err != nil {
			panic(err)
		}
		util.WritePid(conf.Server.PidFile)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	engine := server.NewMomonga(conf)
	setupEngine(engine)

	t := server.NewTcpServer(engine, conf)
	//u := server.NewUnixServer(engine, conf)
	h := server.NewHttpServer(engine, conf)

	app := server.NewApplication(engine)
	app.RegisterServer(t)
	//app.RegisterServer(u)
	app.RegisterServer(h)

	app.Start()
	app.Loop()

	log.Info("Server finished")
}
