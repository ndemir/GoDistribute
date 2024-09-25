package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Server struct {
	Hostname   string `yaml:"hostname"`
	Port       int    `yaml:"port"`
	Username   string `yaml:"username"`
	SSHKeyFile string `yaml:"ssh_key_file"`
	HomeDir    string
}

type Config struct {
	Servers []Server `yaml:"servers"`
}

func LoadServers(configFile string) ([]Server, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return config.Servers, nil
}
