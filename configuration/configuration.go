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
	Bridge Bridge `toml:"bridge"`
}

type Engine struct {
	QueueSize         int             `toml:"queue_size"`
	AcceptorCount     string          `toml:"acceptor_count"`
	LockPoolSize      int             `toml:"lock_pool_size"`
	EnableSys         bool            `toml:"enable_sys"`
	FanoutWorkerCount string          `toml:"fanout_worker_count"`
	AllowAnonymous    bool            `toml:"allow_anonymous"`
	Authenticators    []Authenticator `toml:"authenticator"`
	EnablePermission  bool            `toml:"enable_permission"`
}

type Authenticator struct {
	Type string `toml:"type"`
}

type Bridge struct {
	Address string `toml:"address"`
	Port int `toml:"port"`
	CleanSession bool `toml:"clean_session"`
	ClientId string `toml:"client_id"`
	KeepaliveInterval int `toml:"keepalive_interval"`
	Connection string `toml:"connection"`
	Type string `toml:"type"`
}

type Server struct {
	User                string `toml:"user"`
	LogFile             string `toml:"log_file"`
	LogLevel            string `toml:"log_level"`
	PidFile             string `toml:"pid_file"`
	BindAddress         string `toml:"bind_address"`
	Port                int    `toml:"port"`
	EnableTls          bool    `toml:"enable_tls"`
	TlsPort             int    `toml:"tls_port"`
	Keyfile             string `toml:"keyfile"`
	Cafile            string `toml:"cafile"`
	Certfile            string `toml:"certfile"`
	Socket              string `toml:"socket"`
	HttpPort            int    `toml:"http_port"`
	WebSocketMount      string `toml:"websocket_mount"`
	HttpDebug           bool   `toml:"http_debug"`
	MaxInflightMessages int    `toml:"max_inflight_messages"`
	MaxQueuedMessages   int    `toml:"max_queued_messages"`
	RetryInterval       int    `toml:"retry_interval"`
	MessageSizeLimit    int    `toml:"message_size_limit"`
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

func (self *Config) GetTlsListenAddress() string {
	if self.Server.TlsPort <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d", self.Server.BindAddress, self.Server.TlsPort)
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

func (self *Config) GetAuthenticators() []Authenticator {
	return self.Engine.Authenticators
}

func DefaultConfiguration() *Config {
	return &Config{
		Engine: Engine{
			QueueSize:         8192,
			AcceptorCount:     "cpu",
			FanoutWorkerCount: "cpu",
			LockPoolSize:      64,
			EnableSys:         true,
			AllowAnonymous:    true,
			EnablePermission:  false,
		},
		Server: Server{
			User:                "momonga",
			LogFile:             "stdout",
			LogLevel:            "debug",
			PidFile:             "",
			BindAddress:         "localhost",
			Port:                1883,
			Socket:              "",
			HttpPort:            0,
			WebSocketMount:      "/mqtt",
			HttpDebug:           true,
			MaxInflightMessages: 10000,
			MaxQueuedMessages:   10000,
			RetryInterval:       20,
			MessageSizeLimit:    8192,
			EnableTls: false,
			TlsPort: 8883,
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

func LoadConfigurationTo(configFile string, to *Config) error {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return err
	}

	if _, err2 := toml.Decode(string(data), to); err != nil {
		return err2
	}

	return nil
}
