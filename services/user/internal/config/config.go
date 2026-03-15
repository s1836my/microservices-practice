package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	Telemetry TelemetryConfig
	Log       LogConfig
}

type ServerConfig struct {
	GRPCPort         int
	HTTPPort         int
	EnableReflection bool
}

type DatabaseConfig struct {
	Host         string
	Port         int
	Name         string
	User         string
	Password     string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
}

type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
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

const minJWTSecretLen = 32

func Load() (*Config, error) {
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("grpc_port", 50051)
	v.SetDefault("http_port", 8080)
	v.SetDefault("enable_reflection", false)
	v.SetDefault("db_host", "localhost")
	v.SetDefault("db_port", 5432)
	v.SetDefault("db_sslmode", "disable") // 開発: disable, 本番: require
	v.SetDefault("db_max_open_conns", 25)
	v.SetDefault("db_max_idle_conns", 5)
	v.SetDefault("jwt_access_ttl", "1h")
	v.SetDefault("jwt_refresh_ttl", "720h")
	v.SetDefault("telemetry_sample_rate", 1.0)
	v.SetDefault("telemetry_service_name", "user-service")
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "json")

	// 必須パラメータのバリデーション
	jwtSecret := v.GetString("jwt_secret")
	if len(jwtSecret) < minJWTSecretLen {
		return nil, fmt.Errorf("JWT_SECRET must be at least %d characters", minJWTSecretLen)
	}

	dbName := v.GetString("db_name")
	if dbName == "" {
		return nil, fmt.Errorf("DB_NAME is required")
	}

	dbUser := v.GetString("db_user")
	if dbUser == "" {
		return nil, fmt.Errorf("DB_USER is required")
	}

	dbPassword := v.GetString("db_password")
	if dbPassword == "" {
		return nil, fmt.Errorf("DB_PASSWORD is required")
	}

	accessTTL, err := time.ParseDuration(v.GetString("jwt_access_ttl"))
	if err != nil {
		return nil, fmt.Errorf("parse JWT_ACCESS_TTL: %w", err)
	}

	refreshTTL, err := time.ParseDuration(v.GetString("jwt_refresh_ttl"))
	if err != nil {
		return nil, fmt.Errorf("parse JWT_REFRESH_TTL: %w", err)
	}

	return &Config{
		Server: ServerConfig{
			GRPCPort:         v.GetInt("grpc_port"),
			HTTPPort:         v.GetInt("http_port"),
			EnableReflection: v.GetBool("enable_reflection"),
		},
		Database: DatabaseConfig{
			Host:         v.GetString("db_host"),
			Port:         v.GetInt("db_port"),
			Name:         dbName,
			User:         dbUser,
			Password:     dbPassword,
			SSLMode:      v.GetString("db_sslmode"),
			MaxOpenConns: v.GetInt("db_max_open_conns"),
			MaxIdleConns: v.GetInt("db_max_idle_conns"),
		},
		JWT: JWTConfig{
			Secret:     jwtSecret,
			AccessTTL:  accessTTL,
			RefreshTTL: refreshTTL,
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

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s pool_max_conns=%d",
		c.Host, c.Port, c.Name, c.User, c.Password, c.SSLMode, c.MaxOpenConns,
	)
}
