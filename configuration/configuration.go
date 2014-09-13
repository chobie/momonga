// Copyright 2014, Shuhei Tanuma. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package configuration

import (
	"fmt"
	"io/ioutil"
	"github.com/BurntSushi/toml"
)

type Config struct {
	Server         Server       `toml:"server"`
}

type Server struct {
	LogFile  string   `toml:"log_file"`
	LogLevel string   `toml:"log_level"`
	PidFile  string   `toml:"pid_file"`
	BindAddress string `toml:"bind_address"`
	Port int `toml:"port"`
	Socket string `toml:"socket"`
	EnableSys bool `toml:"enable_sys"`
	HttpPort int `toml:"http_port"`
	WebSocketMount string `toml:"websocket_mount"`
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

func LoadConfiguration(configFile string) (*Config, error) {
	config := &Config{
		Server: Server{
			LogFile:  "stderr",
			LogLevel: "info",
			PidFile:  "",
			BindAddress: "localhost",
			Port: 1883,
			Socket: "",
			EnableSys: true,
			HttpPort: 9000,
			WebSocketMount: "/mqtt",
		},
	}

	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return config, err
	}

	if _, err2 := toml.Decode(string(data), config); err != nil {
		return config, err2
	}

	return config, nil
}
