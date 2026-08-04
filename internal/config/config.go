package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath  = "config/config.yaml"
	minJWTSecretLength = 32
)

type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	parsed, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", value.Value, err)
	}

	*d = Duration(parsed)

	return nil
}

func (d Duration) Std() time.Duration { return time.Duration(d) }

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	DB        DBConfig        `yaml:"db"`
	Redis     RedisConfig     `yaml:"redis"`
	JWT       JWTConfig       `yaml:"jwt"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Cache     CacheConfig     `yaml:"cache"`
	Email     EmailConfig     `yaml:"email"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type DBConfig struct {
	DSN             string   `yaml:"dsn"`
	MaxOpenConns    int      `yaml:"max_open_conns"`
	MaxIdleConns    int      `yaml:"max_idle_conns"`
	ConnMaxLifetime Duration `yaml:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type JWTConfig struct {
	Secret string   `yaml:"secret"`
	TTL    Duration `yaml:"ttl"`
}

type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
}

type CacheConfig struct {
	TasksTTL Duration `yaml:"tasks_ttl"`
}

type EmailConfig struct {
	BaseURL string   `yaml:"base_url"`
	Timeout Duration `yaml:"timeout"`
}

func Load() (Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = defaultConfigPath
	}

	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config file %s: %w", path, err)
	}

	applyEnvOverrides(&cfg)
	applyDefaults(&cfg)

	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SERVER_ADDR"); v != "" {
		cfg.Server.Addr = v
	}

	if v := os.Getenv("DB_DSN"); v != "" {
		cfg.DB.DSN = v
	}

	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}

	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}

	if v := os.Getenv("REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Redis.DB = n
		}
	}

	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}

	if v := os.Getenv("EMAIL_BASE_URL"); v != "" {
		cfg.Email.BaseURL = v
	}
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}

	if cfg.DB.MaxOpenConns <= 0 {
		cfg.DB.MaxOpenConns = 25
	}

	if cfg.DB.MaxIdleConns <= 0 {
		cfg.DB.MaxIdleConns = 10
	}

	if cfg.DB.ConnMaxLifetime <= 0 {
		cfg.DB.ConnMaxLifetime = Duration(5 * time.Minute)
	}

	if cfg.JWT.TTL <= 0 {
		cfg.JWT.TTL = Duration(24 * time.Hour)
	}

	if cfg.RateLimit.RequestsPerMinute <= 0 {
		cfg.RateLimit.RequestsPerMinute = 100
	}

	if cfg.Cache.TasksTTL <= 0 {
		cfg.Cache.TasksTTL = Duration(5 * time.Minute)
	}

	if cfg.Email.Timeout <= 0 {
		cfg.Email.Timeout = Duration(3 * time.Second)
	}
}

func (c Config) validate() error {
	if c.DB.DSN == "" {
		return fmt.Errorf("db.dsn is required (set it in the config file or via DB_DSN)")
	}

	if c.Redis.Addr == "" {
		return fmt.Errorf("redis.addr is required (set it in the config file or via REDIS_ADDR)")
	}

	if len(c.JWT.Secret) < minJWTSecretLength {
		return fmt.Errorf(
			"jwt secret must be at least %d characters; set a strong value via JWT_SECRET",
			minJWTSecretLength,
		)
	}

	if c.Email.BaseURL == "" {
		return fmt.Errorf("email.base_url is required (set it in the config file or via EMAIL_BASE_URL)")
	}

	return nil
}
