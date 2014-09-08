package main

import (
	"flag"
	"github.com/chobie/momonga/util"
	"github.com/chobie/momonga/server"
	log "github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/configuration"
	"os"
	"strconv"
	"io/ioutil"
	"runtime"
)

func main() {
	foreGround := flag.Bool("foreground", true, "run as foreground")
	configFile := flag.String("config", "config.toml", "the config file")
	pidFile := flag.String("pidfile", "", "the pid file")
	flag.Parse()

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
	svr, _ := server.NewMomongaServer(conf)
	svr.ListenAndServe()
	select{}
}
