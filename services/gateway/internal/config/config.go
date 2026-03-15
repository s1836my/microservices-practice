package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

const minJWTSecretLen = 32

type Config struct {
	Server    ServerConfig
	Services  ServicesConfig
	JWT       JWTConfig
	RateLimit RateLimitConfig
	Telemetry TelemetryConfig
	Log       LogConfig
}

type ServerConfig struct {
	HTTPPort int
	GinMode  string
}

type ServicesConfig struct {
	UserAddr    string
	ProductAddr string
	SearchAddr  string
	CartAddr    string
	OrderAddr   string
}

type JWTConfig struct {
	Secret string
}

type RateLimitConfig struct {
	RequestsPerSecond float64
	Burst             int
}

type TelemetryConfig struct {
	Endpoint    string
	ServiceName string
	SampleRate  float64
}

type LogConfig struct {
	Level  string
	Format string
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("http_port", 8080)
	v.SetDefault("gin_mode", "release")
	v.SetDefault("user_service_addr", "localhost:50051")
	v.SetDefault("product_service_addr", "localhost:50052")
	v.SetDefault("search_service_addr", "localhost:50053")
	v.SetDefault("cart_service_addr", "localhost:50054")
	v.SetDefault("order_service_addr", "localhost:50055")
	v.SetDefault("rate_limit_rps", 100.0)
	v.SetDefault("rate_limit_burst", 200)
	v.SetDefault("telemetry_sample_rate", 1.0)
	v.SetDefault("telemetry_service_name", "gateway")
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "json")

	jwtSecret := v.GetString("jwt_secret")
	if len(jwtSecret) < minJWTSecretLen {
		return nil, fmt.Errorf("JWT_SECRET must be at least %d characters", minJWTSecretLen)
	}

	return &Config{
		Server: ServerConfig{
			HTTPPort: v.GetInt("http_port"),
			GinMode:  v.GetString("gin_mode"),
		},
		Services: ServicesConfig{
			UserAddr:    v.GetString("user_service_addr"),
			ProductAddr: v.GetString("product_service_addr"),
			SearchAddr:  v.GetString("search_service_addr"),
			CartAddr:    v.GetString("cart_service_addr"),
			OrderAddr:   v.GetString("order_service_addr"),
		},
		JWT: JWTConfig{
			Secret: jwtSecret,
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: v.GetFloat64("rate_limit_rps"),
			Burst:             v.GetInt("rate_limit_burst"),
		},
		Telemetry: TelemetryConfig{
			Endpoint:    v.GetString("telemetry_endpoint"),
			ServiceName: v.GetString("telemetry_service_name"),
			SampleRate:  v.GetFloat64("telemetry_sample_rate"),
		},
		Log: LogConfig{
			Level:  v.GetString("log_level"),
			Format: v.GetString("log_format"),
		},
	}, nil
}
