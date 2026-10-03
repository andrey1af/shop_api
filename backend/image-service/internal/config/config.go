package config

import "github.com/ilyakaznacheev/cleanenv"

type Config struct {
	Env    string `env:"ENV" env-default:"local"`
	GRPC   GRPCConfig
	Shards ShardsConfig
}

type MigratorConfig struct {
	Shards        ShardsConfig
	MigrationsDir string `env:"DB_MIGRATIONS_DIR" env-required:"true"`
}

type GRPCConfig struct {
	Port            string `env:"GRPC_PORT" env-default:"50052"`
	MaxMessageBytes int    `env:"GRPC_MAX_MESSAGE_BYTES" env-default:"26214400"`
}

type ShardsConfig struct {
	Shard1URL string `env:"DB_SHARD_1_URL" env-required:"true"`
	Shard2URL string `env:"DB_SHARD_2_URL" env-required:"true"`
	Shard3URL string `env:"DB_SHARD_3_URL" env-required:"true"`
	Shard4URL string `env:"DB_SHARD_4_URL" env-required:"true"`
}

func (c ShardsConfig) URLs() []string {
	return []string{c.Shard1URL, c.Shard2URL, c.Shard3URL, c.Shard4URL}
}

func MustLoad() *Config {
	var cfg Config

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		panic("failed to read config: " + err.Error())
	}

	return &cfg
}

func MustLoadMigrator() *MigratorConfig {
	var cfg MigratorConfig

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		panic("failed to read config: " + err.Error())
	}

	return &cfg
}
