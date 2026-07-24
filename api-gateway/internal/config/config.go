package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv   string
	HTTPAddr string
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:   strings.TrimSpace(os.Getenv("APP_ENV")),
		HTTPAddr: strings.TrimSpace(os.Getenv("HTTP_ADDR")),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func loadDotEnv() error {
	err := godotenv.Load()
	if err == nil {
		return nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return fmt.Errorf("load .env: %w", err)
}

func (c Config) validate() error {
	if c.AppEnv == "" {
		return errors.New("APP_ENV is required")
	}

	if c.HTTPAddr == "" {
		return errors.New("HTTP_ADDR is required")
	}

	return nil
}
