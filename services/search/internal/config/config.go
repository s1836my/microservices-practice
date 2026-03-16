package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server        ServerConfig
	Elasticsearch ElasticsearchConfig
	Kafka         KafkaConfig
	Telemetry     TelemetryConfig
	Log           LogConfig
}

type ServerConfig struct {
	GRPCPort         int
	HTTPPort         int
	EnableReflection bool
}

type ElasticsearchConfig struct {
	URL   string
	Index string
}

type KafkaConfig struct {
	Brokers []string
	Topic   string
	GroupID string
	Enabled bool
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

// Load reads Search Service configuration from environment variables.
func Load() (*Config, error) {
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("grpc_port", 50053)
	v.SetDefault("http_port", 8082)
	v.SetDefault("enable_reflection", false)

	v.SetDefault("elasticsearch_url", "http://localhost:9200")
	v.SetDefault("elasticsearch_index", "products")

	v.SetDefault("kafka_topic", "product.events")
	v.SetDefault("kafka_group_id", "search-service")
	v.SetDefault("kafka_enabled", true)

	v.SetDefault("telemetry_sample_rate", 1.0)
	v.SetDefault("telemetry_service_name", "search-service")

	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "json")

	brokers := []string{"localhost:9092"}
	if raw := strings.TrimSpace(v.GetString("kafka_brokers")); raw != "" {
		brokers = nil
		for _, broker := range strings.Split(raw, ",") {
			broker = strings.TrimSpace(broker)
			if broker != "" {
				brokers = append(brokers, broker)
			}
		}
	}

	return &Config{
		Server: ServerConfig{
			GRPCPort:         v.GetInt("grpc_port"),
			HTTPPort:         v.GetInt("http_port"),
			EnableReflection: v.GetBool("enable_reflection"),
		},
		Elasticsearch: ElasticsearchConfig{
			URL:   v.GetString("elasticsearch_url"),
			Index: v.GetString("elasticsearch_index"),
		},
		Kafka: KafkaConfig{
			Brokers: brokers,
			Topic:   v.GetString("kafka_topic"),
			GroupID: v.GetString("kafka_group_id"),
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
