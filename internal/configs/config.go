package configs

import (
	"fmt"
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
		DB       int    `yaml:"redis_db"`
	} `yaml:"redis"`

	Mail struct {
		SMTPHost     string `mapstructure:"smtpHost"`
		SMTPPort     string `mapstructure:"smtpPort"`
		SMTPUsername string `mapstructure:"smtpUsername"`
		SMTPPassword string `mapstructure:"smtpPassword"`
		SenderEmail  string
		EmailAPIKey  string
	}

	Env struct {
		CurrentEnv string `mapstructure:"currentEnv"`
		BaseAPIUrl string `mapstructure:"baseAPIUrl"`
	}

	Providers struct {
		GoogleClientID     string `mapstructure:"googleClientID"`
		GoogleClientSecret string `mapstructure:"googleClientSecret"`
		FBClientID         string `mapstructure:"fbClientID"`
		FBClientSecret     string `mapstructure:"fbClientSecret"`
	}
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

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	cfg.DB.Password = getDBPassword(env)
	cfg.Redis.Password = getRedisPassword()
	cfg.Redis.DB = 0

	cfg.Mail.SMTPHost = os.Getenv("SMTP_HOST")
	cfg.Mail.SMTPPort = os.Getenv("SMTP_PORT")
	cfg.Mail.SMTPUsername = os.Getenv("SMTP_USERNAME")
	cfg.Mail.SMTPPassword = os.Getenv("SMTP_PASSWORD")
	cfg.Mail.EmailAPIKey = os.Getenv("EMAIL_API_KEY")
	cfg.Mail.SenderEmail = os.Getenv("SENDER_EMAIL")
	cfg.Providers.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	cfg.Providers.GoogleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	cfg.Providers.FBClientID = os.Getenv("FACEBOOK_CLIENT_ID")
	cfg.Providers.FBClientSecret = os.Getenv("FACEBOOK_CLIENT_SECRET")

	cfg.Env.CurrentEnv = os.Getenv("APP_ENV")
	cfg.Env.BaseAPIUrl = os.Getenv("PRO_BASE_API_URL")

	expandConfig(&cfg, env)

	return &cfg, nil
}

func (c *Config) SQL_DSB() string {
	if c.Env.CurrentEnv == "production" {
		urlString := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&timeout=30s", c.DB.User, c.DB.Password, c.DB.Host, c.DB.Port, c.DB.Name)
		return urlString
	}
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true",
		c.DB.User, c.DB.Password, c.DB.Host, 3306, c.DB.Name,
	) // We change the PORT to 3306 when connecting via Docker instead of c.DB.Port
}

func getDBPassword(env string) string {
	var (
		envVarName string
		secretFile string
	)

	switch env {
	case "production":
		envVarName = "PROD_DB_PASSWORD"
		secretFile = ".prod_db_password"
	default:
		envVarName = "DEV_DB_PASSWORD"
		secretFile = ".dev_db_password"
	}

	password := ""

	if pass := os.Getenv(envVarName); pass != "" {
		password = pass
	}

	if password == "" {
		if err := godotenv.Load(); err == nil {
			if pass := os.Getenv(envVarName); pass != "" {
				password = pass
			}
		}
	}

	if password == "" {
		secretPath := filepath.Join("..", "secrets", secretFile)
		if data, err := os.ReadFile(secretPath); err == nil {
			password = strings.TrimSpace(string(data))
		}
	}

	return password
}

func getRedisPassword() string {
	envVarName := "REDIS_PASSWORD"
	secretFile := ".redis_password"

	password := ""

	if pass := os.Getenv(envVarName); pass != "" {
		password = pass
	}

	if password == "" {
		if err := godotenv.Load(); err == nil {
			if pass := os.Getenv(envVarName); pass != "" {
				password = pass
			}
		}
	}

	if password == "" {
		secretPath := filepath.Join("..", "secrets", secretFile)
		if data, err := os.ReadFile(secretPath); err == nil {
			password = strings.TrimSpace(string(data))
		}
	}

	return password
}

func expandConfig(cfg *Config, env string) {
	dbPassVar := "DEV_DB_PASSWORD"
	if env == "production" {
		dbPassVar = "PROD_DB_PASSWORD"
	}

	cfg.DB.Password = os.Expand(cfg.DB.Password, func(key string) string {
		if key == "DB_PASSWORD" {
			return os.Getenv(dbPassVar)
		}
		return os.Getenv(key)
	})

	cfg.DB.MySQLDSN = os.Expand(cfg.DB.MySQLDSN, func(key string) string {
		if key == "DB_PASSWORD" {
			return os.Getenv(dbPassVar)
		}
		return os.Getenv(key)
	})

	cfg.Redis.Password = os.ExpandEnv(cfg.Redis.Password)
}
