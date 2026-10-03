package config

import (
	"net"
	"net/url"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env      string `env:"ENV" env-default:"local"`
	HTTP     HTTPConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Image    ImageConfig
	Kafka    KafkaConfig
	Redis    RedisConfig
}

type MigratorConfig struct {
	Database      DatabaseConfig
	MigrationsDir string `env:"DB_MIGRATIONS_DIR" env-required:"true"`
}

type HTTPConfig struct {
	Port              string        `env:"HTTP_PORT" env-default:"8090"`
	ReadHeaderTimeout time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" env-default:"5s"`
	ShutdownTimeout   time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" env-default:"10s"`
	HealthTimeout     time.Duration `env:"HTTP_HEALTH_TIMEOUT" env-default:"2s"`

	RefreshCookieSecure bool `env:"HTTP_REFRESH_COOKIE_SECURE" env-default:"false"`

	AuthRateLimitPerMinute int `env:"HTTP_AUTH_RATE_LIMIT_PER_MINUTE" env-default:"10"`
	AuthRateLimitBurst     int `env:"HTTP_AUTH_RATE_LIMIT_BURST" env-default:"5"`
}

type DatabaseConfig struct {
	URL      string
	Host     string `env:"DB_HOST" env-required:"true"`
	Port     string `env:"DB_PORT" env-required:"true"`
	Name     string `env:"DB_NAME" env-required:"true"`
	User     string `env:"DB_USER" env-required:"true"`
	Password string `env:"DB_PASSWORD" env-required:"true"`
	SSLMode  string `env:"DB_SSL_MODE" env-required:"true"`
}

type AuthConfig struct {
	Addr    string        `env:"AUTH_GRPC_ADDR" env-default:"auth:50051"`
	Timeout time.Duration `env:"AUTH_GRPC_TIMEOUT" env-default:"5s"`
}

type ImageConfig struct {
	Addr            string        `env:"IMAGE_GRPC_ADDR" env-default:"image:50052"`
	Timeout         time.Duration `env:"IMAGE_GRPC_TIMEOUT" env-default:"5s"`
	MaxMessageBytes int           `env:"IMAGE_GRPC_MAX_MESSAGE_BYTES" env-default:"26214400"`
}

type KafkaConfig struct {
	Brokers             []string `env:"KAFKA_BROKERS" env-separator:"," env-default:"kafka:9092"`
	ProductUpdatesTopic string   `env:"KAFKA_PRODUCT_UPDATES_TOPIC" env-default:"product.updates"`
	ConsumerGroupID     string   `env:"KAFKA_CONSUMER_GROUP_ID" env-default:"api-service"`
}

type RedisConfig struct {
	Addr          string        `env:"REDIS_ADDR"`
	Password      string        `env:"REDIS_PASSWORD"`
	DB            int           `env:"REDIS_DB" env-default:"0"`
	ImageTTL      time.Duration `env:"REDIS_IMAGE_TTL" env-default:"1h"`
	ImageMaxBytes int           `env:"REDIS_IMAGE_MAX_BYTES" env-default:"5242880"`
}

func LoadConfig() (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, err
	}

	cfg.Database.URL = cfg.Database.buildURL()

	return &cfg, nil
}

func LoadMigratorConfig() (*MigratorConfig, error) {
	var cfg MigratorConfig
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, err
	}

	cfg.Database.URL = cfg.Database.buildURL()

	return &cfg, nil
}

func (c DatabaseConfig) buildURL() string {
	query := url.Values{"sslmode": []string{c.SSLMode}}
	databaseURL := url.URL{
		Scheme:   "postgres",
		Host:     net.JoinHostPort(c.Host, c.Port),
		Path:     "/" + c.Name,
		RawQuery: query.Encode(),
		User:     url.UserPassword(c.User, c.Password),
	}

	return databaseURL.String()
}
