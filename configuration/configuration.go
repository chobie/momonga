// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package configuration

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"io/ioutil"
	"runtime"
	"strconv"
	"strings"
)

type Config struct {
	Server Server `toml:"server"`
	Engine Engine `toml:"engine"`
}

type Engine struct {
	QueueSize         int    `toml:queue_size`
	AcceptorCount     string `toml:acceptor_count`
	LockPoolSize      int    `toml:lock_pool_size`
	EnableSys         bool   `toml:"enable_sys"`
	FanoutWorkerCount string `toml:fanout_worker_count`
}

type Server struct {
	LogFile        string `toml:"log_file"`
	LogLevel       string `toml:"log_level"`
	PidFile        string `toml:"pid_file"`
	BindAddress    string `toml:"bind_address"`
	Port           int    `toml:"port"`
	Socket         string `toml:"socket"`
	HttpPort       int    `toml:"http_port"`
	WebSocketMount string `toml:"websocket_mount"`
}

func (self *Config) GetQueueSize() int {
	return self.Engine.QueueSize
}

func (self *Config) GetFanoutWorkerCount() int {
	if strings.ToLower(self.Engine.FanoutWorkerCount) == "cpu" {
		return runtime.NumCPU()
	} else {
		v, e := strconv.ParseInt(self.Engine.FanoutWorkerCount, 10, 64)
		if e != nil {
			return 1
		}
		return int(v)
	}
}

func (self *Config) GetLockPoolSize() int {
	return self.Engine.LockPoolSize
}

func (self *Config) GetAcceptorCount() int {
	if strings.ToLower(self.Engine.AcceptorCount) == "cpu" {
		return runtime.NumCPU()
	} else {
		v, e := strconv.ParseInt(self.Engine.AcceptorCount, 10, 64)
		if e != nil {
			return 1
		}
		return int(v)
	}
}

func (self *Config) GetListenAddress() string {
	if self.Server.Port <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d", self.Server.BindAddress, self.Server.Port)
}

func (self *Config) GetSSLListenAddress() string {
	if self.Server.Port <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:8883", self.Server.BindAddress)
}

func (self *Config) GetSocketAddress() string {
	return self.Server.Socket
}

func DefaultConfiguration() *Config {
	return &Config{
		Engine: Engine{
			QueueSize:         8192,
			AcceptorCount:     "cpu",
			FanoutWorkerCount: "cpu",
			LockPoolSize:      64,
			EnableSys:         true,
		},
		Server: Server{
			LogFile:        "stdout",
			LogLevel:       "debug",
			PidFile:        "",
			BindAddress:    "localhost",
			Port:           1883,
			Socket:         "",
			HttpPort:       9000,
			WebSocketMount: "/mqtt",
		},
	}
}

func LoadConfiguration(configFile string) (*Config, error) {
	config := DefaultConfiguration()

	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return config, err
	}

	if _, err2 := toml.Decode(string(data), config); err != nil {
		return config, err2
	}

	return config, nil
}
