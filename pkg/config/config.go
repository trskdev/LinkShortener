package config

import (
	"fmt"
	"log"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	Env   string       `env:"ENV" env-default:"dev"`
	DB    DBConfig     `env-prefix:"DB_"`
	Srv   ServerConfig `env-prefix:"SRV_"`
	Redis RedisConfig  `env-prefix:"REDIS_"`
}

type DBConfig struct {
	User     string `env:"USER"`
	Password string `env:"PWD"`
	Host     string `env:"HOST" env-default:"localhost"`
	Port     string `env:"PORT" env-default:"5432"`
	DBName   string `env:"NAME"`
}

func (dbc DBConfig) GetDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%v/%s?sslmode=disable",
		dbc.User,
		dbc.Password,
		dbc.Host,
		dbc.Port,
		dbc.DBName,
	)
}

type ServerConfig struct {
	Host         string        `env:"HOST" env-default:"0.0.0.0"`
	Port         string        `env:"PORT" env-default:"8080"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT" env-default:"5s"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT" env-default:"10s"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT" env-default:"60s"`
}

type RedisConfig struct {
	Password string `env:"PWD"`
	Host     string `env:"HOST" env-default:"localhost"`
	Port     string `env:"PORT" env-default:"6379"`
	DB       int    `env:"DB" env-default:"0"`
}

func (sc ServerConfig) GetAddr() string {
	return fmt.Sprintf("%s:%s", sc.Host, sc.Port)
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		_ = godotenv.Load("../../.env")
		log.Println("failed to find the .env file, data will be read from OS")
	}

	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		log.Fatalf("failed to read the config: %s", err.Error())
	}

	return &cfg
}
