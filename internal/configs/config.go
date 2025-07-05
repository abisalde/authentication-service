package configs

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	DB struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		MySQLDSN string `yaml:"mysql_dsn"`
		Name     string `yaml:"dbname"`
		SSLMode  string `yaml:"sslmode"`
		Migrate  bool   `yaml:"migrate"`
	} `yaml:"database"`

	Redis struct {
		Addr     string `yaml:"redis_addr"`
		Password string `yaml:"redis_password"`
	} `yaml:"redis"`
}

func Load(env string) (*Config, error) {
	var cfg Config
	configFile := "dev.yml"

	if env == "production" {
		configFile = "prod.yml"
	}

	configPath := filepath.Join("internal", "configs", configFile)
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	log.Printf("Loading config from: %s", configPath)

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	// Load environment-specific password
	cfg.DB.Password = getPassword(env)

	// Expand all environment variables in the config
	expandConfig(&cfg, env)

	log.Printf("This is the password: %v", cfg.DB.Password)

	return &cfg, nil
}

func (c *Config) SQL_DSB() string {
	log.Printf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		c.DB.User, c.DB.Password, c.DB.Host, c.DB.Port, c.DB.Name)
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true",
		c.DB.User, c.DB.Password, c.DB.Host, c.DB.Port, c.DB.Name,
	)
}

func getPassword(env string) string {
	// Determine password variable and secret file based on environment
	var (
		envVarName string
		secretFile string
	)

	switch env {
	case "production":
		envVarName = "PROD_DB_PASSWORD"
		secretFile = ".prod_db_password"
	default: // development
		envVarName = "DEV_DB_PASSWORD"
		secretFile = ".dev_db_password"
	}

	// Password lookup priority:
	// 1. Direct environment variable
	// 2. .env file
	// 3. Environment-specific secrets file
	// (YAML config fallback is handled by os.ExpandEnv later)

	password := ""

	// 1. Check environment variable first
	if pass := os.Getenv(envVarName); pass != "" {
		password = pass
	}

	// 2. Fallback to .env file if not set
	if password == "" {
		if err := godotenv.Load(); err == nil {
			if pass := os.Getenv(envVarName); pass != "" {
				password = pass
			}
		}
	}

	// 3. Fallback to secrets file
	if password == "" {
		secretPath := filepath.Join("..", "secrets", secretFile)
		if data, err := os.ReadFile(secretPath); err == nil {
			password = strings.TrimSpace(string(data))
		}
	}

	return password
}

func expandConfig(cfg *Config, env string) {
	// Determine the appropriate environment variable names
	dbPassVar := "DEV_DB_PASSWORD"
	if env == "production" {
		dbPassVar = "PROD_DB_PASSWORD"
	}

	// Expand DB configuration using the correct variable name
	cfg.DB.Password = os.Expand(cfg.DB.Password, func(key string) string {
		if key == "DB_PASSWORD" { // Matches ${DB_PASSWORD} in YAML
			return os.Getenv(dbPassVar)
		}
		return os.Getenv(key)
	})

	cfg.DB.MySQLDSN = os.Expand(cfg.DB.MySQLDSN, func(key string) string {
		if key == "DB_PASSWORD" { // Matches ${DB_PASSWORD} in YAML
			return os.Getenv(dbPassVar)
		}
		return os.Getenv(key)
	})

	// Expand Redis configuration
	cfg.Redis.Password = os.ExpandEnv(cfg.Redis.Password)
}
