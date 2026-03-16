package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yourorg/micromart/services/cart/internal/config"
)

func TestLoad_UsesDefaults(t *testing.T) {
	t.Setenv("GRPC_PORT", "")
	t.Setenv("HTTP_PORT", "")
	t.Setenv("ENABLE_REFLECTION", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("REDIS_DB", "")
	t.Setenv("TELEMETRY_ENDPOINT", "")
	t.Setenv("TELEMETRY_SERVICE_NAME", "")
	t.Setenv("TELEMETRY_SAMPLE_RATE", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 50054, cfg.Server.GRPCPort)
	assert.Equal(t, 8083, cfg.Server.HTTPPort)
	assert.Equal(t, "localhost:6379", cfg.Redis.Addr)
	assert.Equal(t, "cart-service", cfg.Telemetry.ServiceName)
}

func TestLoad_ReadsEnvironment(t *testing.T) {
	t.Setenv("GRPC_PORT", "60000")
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("ENABLE_REFLECTION", "true")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("REDIS_DB", "5")
	t.Setenv("TELEMETRY_ENDPOINT", "http://jaeger:4318")
	t.Setenv("TELEMETRY_SERVICE_NAME", "custom-cart")
	t.Setenv("TELEMETRY_SAMPLE_RATE", "0.5")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "text")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 60000, cfg.Server.GRPCPort)
	assert.Equal(t, 9090, cfg.Server.HTTPPort)
	assert.True(t, cfg.Server.EnableReflection)
	assert.Equal(t, "redis:6379", cfg.Redis.Addr)
	assert.Equal(t, "secret", cfg.Redis.Password)
	assert.Equal(t, 5, cfg.Redis.DB)
	assert.Equal(t, "custom-cart", cfg.Telemetry.ServiceName)
	assert.Equal(t, "debug", cfg.Log.Level)
}
