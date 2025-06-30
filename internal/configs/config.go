package configs

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DB struct {
		Host        string `yaml:"host"`
		Port        int    `yaml:"port"`
		User        string `yaml:"user"`
		Name        string `yaml:"name"`
		SSLMode     string `yaml:"sslmode"`
		SSLRootCert string `yaml:"sslrootcert"`
		SSLCert     string `yaml:"sslcert"`
		SSLKey      string `yaml:"sslkey"`
		Migrate     bool   `yaml:"migrate"`
	} `yaml:"database"`
}

func Load(env string) (*Config, error) {
	var cfg Config
	configFile := "dev.yml"

	if env == "production" {
		configFile = "prod.yml"
	}

	file, err := os.Open(filepath.Join("internal", "configs", configFile))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
