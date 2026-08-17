package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Значения по умолчанию для некритичных параметров
const (
	defaultReadTimeout     = 10 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultShutdownTimeout = 10 * time.Second
	defaultDBDialect       = "mysql"
	defaultDBMaxOpenConns  = 25
	defaultDBMaxIdleConns  = 10
	defaultDBConnLifetime  = 15 * time.Minute
	defaultJWTLifespan     = 24 * time.Hour
	defaultLogLevel        = "info"
	defaultLogFormat       = "json"
	defaultUserCacheTTL    = 15 * time.Minute
	defaultTaskCacheTTL    = 5 * time.Minute
)

type (
	EntityCacheConfig struct {
		Enabled bool          `yaml:"enabled"`
		TTL     time.Duration `yaml:"ttl"`
	}

	RedisCacheConfig struct {
		User EntityCacheConfig `yaml:"user"`
		Task EntityCacheConfig `yaml:"task"`
	}

	RedisConfig struct {
		Enabled  bool   `yaml:"enabled"`
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	}

	Server struct {
		Host            string        `yaml:"host" env:"SERVER_HOST" env-default:"0.0.0.0"`
		Port            string        `yaml:"port" env:"SERVER_PORT" env-default:"8080"`
		ReadTimeout     time.Duration `yaml:"read_timeout" env:"SERVER_READ_TIMEOUT" env-default:"10s"`
		WriteTimeout    time.Duration `yaml:"write_timeout" env:"SERVER_WRITE_TIMEOUT" env-default:"10s"`
		ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env:"SERVER_SHUTDOWN_TIMEOUT" env-default:"10s"`
	}

	Database struct {
		Dialect            string        `yaml:"dialect"`
		Host               string        `yaml:"host"`
		Port               string        `yaml:"port"`
		User               string        `yaml:"user"`
		Password           string        `yaml:"password"`
		Name               string        `yaml:"name"`
		SSLMode            string        `yaml:"ssl_mode"`
		MaxOpenConnections int           `yaml:"max_open_connections"`
		MaxIdleConnections int           `yaml:"max_idle_connections"`
		ConnectionLifetime time.Duration `yaml:"connection_lifetime"`
	}

	JWT struct {
		Secret   string        `yaml:"secret"`
		Lifespan time.Duration `yaml:"lifespan"`
	}

	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	}

	CORS struct {
		AllowedOrigins []string `yaml:"allowed_origins"`
	}

	Config struct {
		Server        Server           `yaml:"server"`
		Database      Database         `yaml:"database"`
		JWT           JWT              `yaml:"jwt"`
		Logging       Logging          `yaml:"logging"`
		CORS          CORS             `yaml:"cors"`
		Redis         RedisConfig      `yaml:"redis"`
		CacheServices RedisCacheConfig `yaml:"cache-services"`
		Environment   string           `yaml:"environment"`
	}
)

func Load(configPath string) (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	cfg := &Config{}
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config YAML: %w", err)
	}

	cfg.setDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func (c *Config) setDefaults() {
	if c.Server.ReadTimeout <= 0 {
		c.Server.ReadTimeout = defaultReadTimeout
	}
	if c.Server.WriteTimeout <= 0 {
		c.Server.WriteTimeout = defaultWriteTimeout
	}
	if c.Server.ShutdownTimeout <= 0 {
		c.Server.ShutdownTimeout = defaultShutdownTimeout
	}

	if strings.TrimSpace(c.Environment) == "" {
		c.Environment = "development"
	}

	// Database defaults
	if strings.TrimSpace(c.Database.Dialect) == "" {
		c.Database.Dialect = defaultDBDialect
	}
	if c.Database.MaxOpenConnections <= 0 {
		c.Database.MaxOpenConnections = defaultDBMaxOpenConns
	}
	if c.Database.MaxIdleConnections <= 0 {
		c.Database.MaxIdleConnections = defaultDBMaxIdleConns
	}
	if c.Database.ConnectionLifetime <= 0 {
		c.Database.ConnectionLifetime = defaultDBConnLifetime
	}

	// JWT defaults
	if c.JWT.Lifespan <= 0 {
		c.JWT.Lifespan = defaultJWTLifespan
	}

	// Logging defaults
	if strings.TrimSpace(c.Logging.Level) == "" {
		c.Logging.Level = defaultLogLevel
	}
	if strings.TrimSpace(c.Logging.Format) == "" {
		c.Logging.Format = defaultLogFormat
	}

	// Redis Cache defaults
	if c.Redis.Enabled {
		if c.CacheServices.User.Enabled && c.CacheServices.User.TTL <= 0 {
			c.CacheServices.User.TTL = defaultUserCacheTTL
		}
		if c.CacheServices.Task.Enabled && c.CacheServices.Task.TTL <= 0 {
			c.CacheServices.Task.TTL = defaultTaskCacheTTL
		}
	}
}

func (c *Config) Validate() error {
	var errs []string

	if strings.TrimSpace(c.Server.Host) == "" {
		errs = append(errs, "server.host is required")
	}

	if strings.TrimSpace(c.Server.Port) == "" {
		errs = append(errs, "server.port is required")
	}

	// Критичные параметры базы данных
	if strings.TrimSpace(c.Database.Host) == "" {
		errs = append(errs, "database.host is required")
	}
	if strings.TrimSpace(c.Database.Port) == "" {
		errs = append(errs, "database.port is required")
	}
	if strings.TrimSpace(c.Database.User) == "" {
		errs = append(errs, "database.user is required")
	}
	if strings.TrimSpace(c.Database.Name) == "" {
		errs = append(errs, "database.name is required")
	}

	// Критичные параметры JWT
	if strings.TrimSpace(c.JWT.Secret) == "" {
		errs = append(errs, "jwt.secret is required")
	} else if len(c.JWT.Secret) < 8 {
		errs = append(errs, "jwt.secret must be at least 8 characters long")
	}

	// Проверяем Redis только если он включен
	if c.Redis.Enabled {
		if strings.TrimSpace(c.Redis.Addr) == "" {
			errs = append(errs, "redis.addr is required when redis is enabled")
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func (c *Config) ServerAddr() string {
	return fmt.Sprintf("%s:%s", c.Server.Host, c.Server.Port)
}
