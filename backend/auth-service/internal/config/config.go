package config

import (
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env      string `env:"ENV" env-default:"local"`
	GRPC     GRPConfig
	JWT      JWTConfig
	Session  SessionConfig
	Database DatabaseConfig
	OAuth    OAuthConfig
}

type MigratorConfig struct {
	Database      DatabaseConfig
	MigrationsDir string `env:"DB_MIGRATIONS_DIR" env-required:"true"`
}

type GRPConfig struct {
	Port    string        `env:"GRPC_PORT"`
	Timeout time.Duration `env:"GRPC_TIMEOUT"`
}

type JWTConfig struct {
	Secret string `env:"JWT_SECRET" env-required:"true"`

	TokenTTL time.Duration `env:"JWT_TOKEN_TTL" env-default:"15m"`
}

type SessionConfig struct {
	RefreshTTL time.Duration `env:"REFRESH_TOKEN_TTL" env-default:"720h"`

	CleanupInterval time.Duration `env:"SESSION_CLEANUP_INTERVAL" env-default:"1h"`
}

type OAuthConfig struct {
	StateTTL    time.Duration       `env:"OAUTH_STATE_TTL" env-default:"10m"`
	HTTPTimeout time.Duration       `env:"OAUTH_HTTP_TIMEOUT" env-default:"10s"`
	Yandex      OAuthProviderConfig `env-prefix:"OAUTH_YANDEX_"`
}

type OAuthProviderConfig struct {
	ClientID     string `env:"CLIENT_ID"`
	ClientSecret string `env:"CLIENT_SECRET"`
	RedirectURL  string `env:"REDIRECT_URL"`
}

func (c OAuthProviderConfig) Enabled() bool {
	return c.ClientID != ""
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

func MustLoad() *Config {
	var cfg Config

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		panic("failed to read config: " + err.Error())
	}

	if err := cfg.JWT.validate(cfg.Env); err != nil {
		panic("invalid config: " + err.Error())
	}

	cfg.Database.URL = cfg.Database.buildURL()

	return &cfg
}

const (
	envProd              = "prod"
	minProdJWTSecretSize = 32
	exampleJWTSecret     = "change-me-in-production"
)

func (c JWTConfig) validate(env string) error {
	if env != envProd {
		return nil
	}
	if len(c.Secret) < minProdJWTSecretSize || c.Secret == exampleJWTSecret {
		return fmt.Errorf("JWT_SECRET must be a random string of at least %d characters in prod", minProdJWTSecretSize)
	}

	return nil
}

func MustLoadMigrator() *MigratorConfig {
	var cfg MigratorConfig

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		panic("failed to read config: " + err.Error())
	}

	cfg.Database.URL = cfg.Database.buildURL()

	return &cfg
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
