package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env       string `env:"ENV" env-default:"local"`
	Generator GeneratorConfig
	API       APIConfig
	Kafka     KafkaConfig
}

type GeneratorConfig struct {
	Interval          time.Duration `env:"GENERATOR_INTERVAL" env-default:"2s"`
	RefreshInterval   time.Duration `env:"GENERATOR_PRODUCTS_REFRESH_INTERVAL" env-default:"30s"`
	PriceDeltaPercent float64       `env:"GENERATOR_PRICE_DELTA_PERCENT" env-default:"20"`
	StockMax          int64         `env:"GENERATOR_STOCK_MAX" env-default:"100"`
}

type APIConfig struct {
	BaseURL     string        `env:"GENERATOR_API_BASE_URL" env-default:"http://api:8090"`
	Timeout     time.Duration `env:"GENERATOR_API_TIMEOUT" env-default:"5s"`
	Email       string        `env:"GENERATOR_EMAIL" env-required:"true"`
	Password    string        `env:"GENERATOR_PASSWORD" env-required:"true"`
	PhoneNumber string        `env:"GENERATOR_PHONE_NUMBER" env-default:"+70000000000"`
}

type KafkaConfig struct {
	Brokers      []string      `env:"KAFKA_BROKERS" env-separator:"," env-default:"kafka:9092"`
	Topic        string        `env:"KAFKA_PRODUCT_UPDATES_TOPIC" env-default:"product.updates"`
	WriteTimeout time.Duration `env:"KAFKA_WRITE_TIMEOUT" env-default:"5s"`
}

func MustLoad() *Config {
	var cfg Config

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		panic("failed to read config: " + err.Error())
	}

	if err := cfg.validate(); err != nil {
		panic("invalid config: " + err.Error())
	}

	return &cfg
}

func (c *Config) validate() error {
	if c.Generator.Interval <= 0 {
		return fmt.Errorf("GENERATOR_INTERVAL must be positive")
	}
	if c.Generator.RefreshInterval <= 0 {
		return fmt.Errorf("GENERATOR_PRODUCTS_REFRESH_INTERVAL must be positive")
	}
	if c.Generator.PriceDeltaPercent <= 0 || c.Generator.PriceDeltaPercent >= 100 {
		return fmt.Errorf("GENERATOR_PRICE_DELTA_PERCENT must be in (0, 100)")
	}
	if c.Generator.StockMax < 0 {
		return fmt.Errorf("GENERATOR_STOCK_MAX must be zero or greater")
	}
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("KAFKA_BROKERS must not be empty")
	}

	return nil
}
