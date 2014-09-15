// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package main

import (
	"flag"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/server"
	"github.com/chobie/momonga/util"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
)

func main() {
	foreGround := flag.Bool("foreground", true, "run as foreground")
	configFile := flag.String("config", "config.toml", "the config file")
	flag.Parse()

	f, _ := os.Create("profiler")
	pprof.StartCPUProfile(f)
	defer func() {
		pprof.StopCPUProfile()
		os.Exit(0)
	}()

	if !*foreGround {
		err := util.Daemonize(0, 0)
		if err != 0 {
			log.Info("fork failed")
			os.Exit(-1)
		}
	}

	pid := os.Getpid()
	log.Info("Server pid: %d started", pid)

	confpath, _ := filepath.Abs(*configFile)
	runtime.GOMAXPROCS(runtime.NumCPU())
	app := server.NewApplication(confpath)
	app.Start()
	app.Loop()

	log.Info("Server pid: %d finished", pid)
}
