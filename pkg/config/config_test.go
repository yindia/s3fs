package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadEnvConfig(t *testing.T) {
	// Set up a .env file for testing
	os.Setenv("CACHE_USERNAME", "testuser")
	os.Setenv("CACHE_PASSWORD", "testpass")
	os.Setenv("CACHE_HOST", "localhost")
	os.Setenv("CACHE_PORT", "6379")
	os.Setenv("CACHE_TYPE", "redis")

	// Load the config
	config, err := LoadEnvConfig()
	assert.NoError(t, err)
	assert.NotNil(t, config)

	// Validate the loaded config
	assert.Equal(t, "testuser", config.Cache.Username)
	assert.Equal(t, "testpass", config.Cache.Password)
	assert.Equal(t, "localhost", config.Cache.Host)
	assert.Equal(t, uint64(6379), config.Cache.Port)
	assert.Equal(t, "redis", config.Cache.CacheType)

	// Clean up
	os.Unsetenv("CACHE_USERNAME")
	os.Unsetenv("CACHE_PASSWORD")
	os.Unsetenv("CACHE_HOST")
	os.Unsetenv("CACHE_PORT")
	os.Unsetenv("CACHE_TYPE")
}
