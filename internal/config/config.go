package config

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Env  string     `yaml:"env"`
	HTTP HTTPConfig `yaml:"http"`
	Log  LogConfig  `yaml:"log"`
	DB   DBConfig   `yaml:"db"`
}

type HTTPConfig struct {
	Port int `yaml:"port"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

type DBConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	SSLMode  string `yaml:"ssl_mode"`
	DSN      string `yaml:"dsn"`
}

func Load() (Config, error) {
	cfg := Config{
		Env: "local",
		HTTP: HTTPConfig{
			Port: 8080,
		},
		Log: LogConfig{
			Level: "info",
		},
		DB: DBConfig{
			Host:    "localhost",
			Port:    5432,
			User:    "subscriptions",
			Name:    "subscriptions",
			SSLMode: "disable",
		},
	}

	_ = loadDotEnv(".env")

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		if _, err := os.Stat("config.yaml"); err == nil {
			configPath = "config.yaml"
		}
	}

	if configPath != "" {
		content, err := os.ReadFile(configPath)
		if err != nil {
			return Config{}, fmt.Errorf("read config file: %w", err)
		}
		if err := yaml.Unmarshal(content, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config file: %w", err)
		}
	}

	applyEnv(&cfg)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.HTTP.Port)
	}
	if c.DB.DSN == "" {
		missing := make([]string, 0)
		if c.DB.Host == "" {
			missing = append(missing, "DB_HOST")
		}
		if c.DB.Port <= 0 || c.DB.Port > 65535 {
			missing = append(missing, "DB_PORT")
		}
		if c.DB.User == "" {
			missing = append(missing, "DB_USER")
		}
		if c.DB.Name == "" {
			missing = append(missing, "DB_NAME")
		}
		if len(missing) > 0 {
			return fmt.Errorf("missing database configuration: %s", strings.Join(missing, ", "))
		}
	}
	return nil
}

func (c Config) DatabaseURL() string {
	if c.DB.DSN != "" {
		return c.DB.DSN
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.DB.User, c.DB.Password),
		Host:   net.JoinHostPort(c.DB.Host, strconv.Itoa(c.DB.Port)),
		Path:   c.DB.Name,
	}
	q := u.Query()
	q.Set("sslmode", c.DB.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

func applyEnv(cfg *Config) {
	setString := func(env string, dst *string) {
		if value := strings.TrimSpace(os.Getenv(env)); value != "" {
			*dst = value
		}
	}
	setInt := func(env string, dst *int) {
		if value := strings.TrimSpace(os.Getenv(env)); value != "" {
			parsed, err := strconv.Atoi(value)
			if err == nil {
				*dst = parsed
			}
		}
	}

	setString("APP_ENV", &cfg.Env)
	setInt("HTTP_PORT", &cfg.HTTP.Port)
	setString("LOG_LEVEL", &cfg.Log.Level)

	setString("DB_HOST", &cfg.DB.Host)
	setInt("DB_PORT", &cfg.DB.Port)
	setString("DB_USER", &cfg.DB.User)
	setString("DB_PASSWORD", &cfg.DB.Password)
	setString("DB_NAME", &cfg.DB.Name)
	setString("DB_SSLMODE", &cfg.DB.SSLMode)
	setString("DB_DSN", &cfg.DB.DSN)
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}
