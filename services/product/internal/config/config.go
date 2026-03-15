package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the product service.
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Kafka     KafkaConfig
	Telemetry TelemetryConfig
	Log       LogConfig
}

// ServerConfig holds gRPC and HTTP server settings.
type ServerConfig struct {
	GRPCPort         int
	HTTPPort         int
	EnableReflection bool
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host         string
	Port         int
	Name         string
	User         string
	Password     string
	SSLMode      string
	MaxOpenConns int
}

// KafkaConfig holds Kafka producer settings.
type KafkaConfig struct {
	Brokers []string
	Topic   string
	Enabled bool
}

// TelemetryConfig holds OpenTelemetry settings.
type TelemetryConfig struct {
	Endpoint    string
	ServiceName string
	SampleRate  float64
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level  string
	Format string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("grpc_port", 50052)
	v.SetDefault("http_port", 8081)
	v.SetDefault("enable_reflection", false)

	v.SetDefault("db_host", "localhost")
	v.SetDefault("db_port", 5433)
	v.SetDefault("db_sslmode", "disable")
	v.SetDefault("db_max_open_conns", 25)

	v.SetDefault("kafka_topic", "product.events")
	v.SetDefault("kafka_enabled", true)

	v.SetDefault("telemetry_sample_rate", 1.0)
	v.SetDefault("telemetry_service_name", "product-service")

	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "json")

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

	var brokers []string
	if raw := v.GetString("kafka_brokers"); raw != "" {
		for _, b := range strings.Split(raw, ",") {
			b = strings.TrimSpace(b)
			if b != "" {
				brokers = append(brokers, b)
			}
		}
	}
	if len(brokers) == 0 {
		brokers = []string{"localhost:9092"}
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
		},
		Kafka: KafkaConfig{
			Brokers: brokers,
			Topic:   v.GetString("kafka_topic"),
			Enabled: v.GetBool("kafka_enabled"),
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

// DSN returns a PostgreSQL connection string.
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s pool_max_conns=%d",
		c.Host, c.Port, c.Name, c.User, c.Password, c.SSLMode, c.MaxOpenConns,
	)
}
