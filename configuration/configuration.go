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
		fmt.Printf("string: %s\n", string(data))
		return config, err2
	}

	return config, nil
}
