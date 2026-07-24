package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type GRPCTransportSecurity string

const (
	GRPCTransportSecurityTLS      GRPCTransportSecurity = "tls"
	GRPCTransportSecurityInsecure GRPCTransportSecurity = "insecure"
)

type GRPCUpstream struct {
	Name   string
	Target string
}

type Config struct {
	AppEnv string

	HTTPAddr string

	GRPCTransportSecurity GRPCTransportSecurity
	GRPCUpstreams         []GRPCUpstream
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	grpcUpstreams, err := parseGRPCUpstreams(
		os.Getenv("GRPC_UPSTREAMS"),
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"parse GRPC_UPSTREAMS: %w",
			err,
		)
	}

	grpcTransportSecurity := GRPCTransportSecurity(
		strings.ToLower(
			strings.TrimSpace(
				os.Getenv("GRPC_TRANSPORT_SECURITY"),
			),
		),
	)

	if grpcTransportSecurity == "" {
		grpcTransportSecurity = GRPCTransportSecurityTLS
	}

	cfg := Config{
		AppEnv:   strings.TrimSpace(os.Getenv("APP_ENV")),
		HTTPAddr: strings.TrimSpace(os.Getenv("HTTP_ADDR")),

		GRPCTransportSecurity: grpcTransportSecurity,
		GRPCUpstreams:         grpcUpstreams,
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

func parseGRPCUpstreams(value string) ([]GRPCUpstream, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	entries := strings.Split(value, ",")

	upstreams := make([]GRPCUpstream, 0, len(entries))
	names := make(map[string]struct{}, len(entries))

	for index, entry := range entries {
		entry = strings.TrimSpace(entry)

		name, target, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, fmt.Errorf(
				"entry %d %q must use name=target format",
				index+1,
				entry,
			)
		}

		name = strings.TrimSpace(name)
		target = strings.TrimSpace(target)

		if name == "" {
			return nil, fmt.Errorf(
				"entry %d upstream name is required",
				index+1,
			)
		}

		if target == "" {
			return nil, fmt.Errorf(
				"upstream %q target is required",
				name,
			)
		}

		if _, exists := names[name]; exists {
			return nil, fmt.Errorf(
				"duplicate upstream %q",
				name,
			)
		}

		names[name] = struct{}{}

		upstreams = append(
			upstreams,
			GRPCUpstream{
				Name:   name,
				Target: target,
			},
		)
	}

	return upstreams, nil
}

func (c Config) validate() error {
	if c.AppEnv == "" {
		return errors.New("APP_ENV is required")
	}

	if c.HTTPAddr == "" {
		return errors.New("HTTP_ADDR is required")
	}

	switch c.GRPCTransportSecurity {
	case GRPCTransportSecurityTLS:
	case GRPCTransportSecurityInsecure:
	default:
		return fmt.Errorf(
			"unsupported GRPC_TRANSPORT_SECURITY %q",
			c.GRPCTransportSecurity,
		)
	}

	return nil
}
