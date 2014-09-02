package main

import (
	"github.com/chobie/momonga/server"
	"github.com/chobie/momonga/logger"
	"github.com/chobie/momonga/configuration"
	"fmt"
)

func main() {
	logger.SetupLogging("debug", "stdout")

	conf, err := configuration.LoadConfiguration("config.toml")
	if err != nil {
		fmt.Printf("Error: %+v")
	}

	fmt.Printf("Configuration: %+v\n", conf)
	svr := server.NewTcpServer()
	svr.ListenAndServe()
}
