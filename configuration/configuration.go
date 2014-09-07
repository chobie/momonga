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
