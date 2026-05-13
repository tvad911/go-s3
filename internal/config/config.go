package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete application configuration.
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Auth      AuthConfig      `mapstructure:"auth"`
	CORS      CORSConfig      `mapstructure:"cors"`
	Log       LogConfig       `mapstructure:"log"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
	Admin     AdminConfig     `mapstructure:"admin"`
}

type ServerConfig struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	MaxHeaderBytes    int           `mapstructure:"max_header_bytes"`
	TLS               TLSConfig     `mapstructure:"tls"`
}

type TLSConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Cert    string `mapstructure:"cert"`
	Key     string `mapstructure:"key"`
}

type StorageConfig struct {
	DataDir              string `mapstructure:"data_dir"`
	TempDir              string `mapstructure:"temp_dir"`
	MetaDB               string `mapstructure:"meta_db"`
	MaxObjectSize        int64  `mapstructure:"max_object_size"`
	MultipartCleanupDays int    `mapstructure:"multipart_cleanup_days"`
}

type AuthConfig struct {
	RootAccessKey      string `mapstructure:"root_access_key"`
	RootSecretKey      string `mapstructure:"root_secret_key"`
	Region             string `mapstructure:"region"`
	TokenExpiryMinutes int    `mapstructure:"token_expiry_minutes"`
}

type CORSConfig struct {
	GlobalAllowedOrigins []string `mapstructure:"global_allowed_origins"`
	GlobalAllowedMethods []string `mapstructure:"global_allowed_methods"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

type RateLimitConfig struct {
	Enabled           bool    `mapstructure:"enabled"`
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`
	Burst             int     `mapstructure:"burst"`
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

type AdminConfig struct {
	APIEnabled bool   `mapstructure:"api_enabled"`
	PathPrefix string `mapstructure:"path_prefix"`
}

// Load loads the configuration from file and environment variables.
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// Default config values
	setDefaults(v)

	// Environment variables config
	v.SetEnvPrefix("GOS3")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}
	if cfg.Storage.DataDir == "" {
		return fmt.Errorf("storage data directory is required")
	}
	if cfg.Storage.TempDir == "" {
		return fmt.Errorf("storage temp directory is required")
	}
	if cfg.Storage.MetaDB == "" {
		return fmt.Errorf("storage meta database path is required")
	}
	if cfg.Auth.RootAccessKey == "" || cfg.Auth.RootSecretKey == "" {
		return fmt.Errorf("root access key and secret key are required")
	}
	return nil
}
