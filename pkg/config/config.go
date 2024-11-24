package config

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

// Config holds the application configuration
type Config struct {
	Cache CacheConfig
}

// CacheConfig holds the cache connection configuration
type CacheConfig struct {
	Username  string `envconfig:"CACHE_USERNAME" yaml:"username"`
	Password  string `envconfig:"CACHE_PASSWORD" yaml:"password"`
	Host      string `envconfig:"CACHE_HOST" yaml:"host"`
	Port      uint64 `envconfig:"CACHE_PORT" yaml:"port"`
	CacheType string `envconfig:"CACHE_TYPE" default:"memory" yaml:"cache_type"`
}

// LoadEnvConfig loads environment variables into the config struct
func LoadEnvConfig() (*Config, error) {
	var config Config
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found, proceeding with default values")
	}

	if err := envconfig.Process("", &config); err != nil {
		fmt.Println("error loading environment variables: %w", err)
	}
	return &config, nil
}
