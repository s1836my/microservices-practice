package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig
	Redis     RedisConfig
	Telemetry TelemetryConfig
	Log       LogConfig
}

type ServerConfig struct {
	GRPCPort         int
	HTTPPort         int
	EnableReflection bool
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
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

	v.SetDefault("grpc_port", 50054)
	v.SetDefault("http_port", 8083)
	v.SetDefault("enable_reflection", false)

	v.SetDefault("redis_addr", "localhost:6379")
	v.SetDefault("redis_password", "")
	v.SetDefault("redis_db", 0)

	v.SetDefault("telemetry_service_name", "cart-service")
	v.SetDefault("telemetry_sample_rate", 1.0)

	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "json")

	return &Config{
		Server: ServerConfig{
			GRPCPort:         v.GetInt("grpc_port"),
			HTTPPort:         v.GetInt("http_port"),
			EnableReflection: v.GetBool("enable_reflection"),
		},
		Redis: RedisConfig{
			Addr:     v.GetString("redis_addr"),
			Password: v.GetString("redis_password"),
			DB:       v.GetInt("redis_db"),
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
