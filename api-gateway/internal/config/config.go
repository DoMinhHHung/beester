package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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

type HTTPUpstream struct {
	Name   string
	Target string
}

type HTTPRoute struct {
	Method   string
	Pattern  string
	Upstream string
}

type GRPCRoute struct {
	Prefix   string
	Upstream string
}

type JWTConfig struct {
	Enabled                  bool
	HMACSecret               string
	Issuer                   string
	Audience                 string
	UserIDClaim              string
	UserIDHeader             string
	ForwardAuthorization     bool
	PublicPathPrefixes       []string
	PublicGRPCMethodPrefixes []string
	Leeway                   time.Duration
}

type RateLimitConfig struct {
	Enabled         bool
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	Capacity        int
	RefillPerSecond float64
	KeyPrefix       string
	FailOpen        bool
	TrustProxy      bool
}

type Config struct {
	AppEnv string

	HTTPAddr            string
	GRPCAddr            string
	MaxRequestBodyBytes int64
	UpstreamTimeout     time.Duration

	HTTPUpstreams []HTTPUpstream
	HTTPRoutes    []HTTPRoute

	GRPCTransportSecurity GRPCTransportSecurity
	GRPCUpstreams         []GRPCUpstream
	GRPCRoutes            []GRPCRoute

	JWT       JWTConfig
	RateLimit RateLimitConfig
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	grpcUpstreams, err := parseGRPCUpstreams(os.Getenv("GRPC_UPSTREAMS"))
	if err != nil {
		return Config{}, fmt.Errorf("parse GRPC_UPSTREAMS: %w", err)
	}

	httpUpstreams, err := parseHTTPUpstreams(os.Getenv("HTTP_UPSTREAMS"))
	if err != nil {
		return Config{}, fmt.Errorf("parse HTTP_UPSTREAMS: %w", err)
	}

	httpRoutes, err := parseHTTPRoutes(os.Getenv("HTTP_ROUTES"))
	if err != nil {
		return Config{}, fmt.Errorf("parse HTTP_ROUTES: %w", err)
	}

	grpcRoutes, err := parseGRPCRoutes(os.Getenv("GRPC_ROUTES"))
	if err != nil {
		return Config{}, fmt.Errorf("parse GRPC_ROUTES: %w", err)
	}

	grpcTransportSecurity := GRPCTransportSecurity(
		strings.ToLower(strings.TrimSpace(os.Getenv("GRPC_TRANSPORT_SECURITY"))),
	)
	if grpcTransportSecurity == "" {
		grpcTransportSecurity = GRPCTransportSecurityTLS
	}

	jwtEnabled, err := envBool("JWT_ENABLED", false)
	if err != nil {
		return Config{}, err
	}

	jwtForwardAuthorization, err := envBool("JWT_FORWARD_AUTHORIZATION", false)
	if err != nil {
		return Config{}, err
	}

	rateLimitEnabled, err := envBool("RATE_LIMIT_ENABLED", false)
	if err != nil {
		return Config{}, err
	}

	rateLimitFailOpen, err := envBool("RATE_LIMIT_FAIL_OPEN", false)
	if err != nil {
		return Config{}, err
	}

	trustProxy, err := envBool("TRUST_PROXY_HEADERS", false)
	if err != nil {
		return Config{}, err
	}

	redisDB, err := envInt("REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}

	rateCapacity, err := envInt("RATE_LIMIT_CAPACITY", 100)
	if err != nil {
		return Config{}, err
	}

	rateRefill, err := envFloat64("RATE_LIMIT_REFILL_PER_SECOND", 50)
	if err != nil {
		return Config{}, err
	}

	maxRequestBodyBytes, err := envInt64("MAX_REQUEST_BODY_BYTES", 8<<20)
	if err != nil {
		return Config{}, err
	}

	upstreamTimeout, err := envDuration("UPSTREAM_REQUEST_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}

	jwtLeeway, err := envDuration("JWT_LEEWAY", 30*time.Second)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv: strings.TrimSpace(os.Getenv("APP_ENV")),

		HTTPAddr:            strings.TrimSpace(os.Getenv("HTTP_ADDR")),
		GRPCAddr:            strings.TrimSpace(os.Getenv("GRPC_ADDR")),
		MaxRequestBodyBytes: maxRequestBodyBytes,
		UpstreamTimeout:     upstreamTimeout,

		HTTPUpstreams: httpUpstreams,
		HTTPRoutes:    httpRoutes,

		GRPCTransportSecurity: grpcTransportSecurity,
		GRPCUpstreams:         grpcUpstreams,
		GRPCRoutes:            grpcRoutes,

		JWT: JWTConfig{
			Enabled:                  jwtEnabled,
			HMACSecret:               strings.TrimSpace(os.Getenv("JWT_HMAC_SECRET")),
			Issuer:                   strings.TrimSpace(os.Getenv("JWT_ISSUER")),
			Audience:                 strings.TrimSpace(os.Getenv("JWT_AUDIENCE")),
			UserIDClaim:              envString("JWT_USER_ID_CLAIM", "sub"),
			UserIDHeader:             envString("JWT_USER_ID_HEADER", "X-User-ID"),
			ForwardAuthorization:     jwtForwardAuthorization,
			PublicPathPrefixes:       envList("JWT_PUBLIC_PATH_PREFIXES", []string{"/healthz", "/readyz"}),
			PublicGRPCMethodPrefixes: envList("JWT_PUBLIC_GRPC_METHOD_PREFIXES", nil),
			Leeway:                   jwtLeeway,
		},

		RateLimit: RateLimitConfig{
			Enabled:         rateLimitEnabled,
			RedisAddr:       strings.TrimSpace(os.Getenv("REDIS_ADDR")),
			RedisPassword:   os.Getenv("REDIS_PASSWORD"),
			RedisDB:         redisDB,
			Capacity:        rateCapacity,
			RefillPerSecond: rateRefill,
			KeyPrefix:       envString("RATE_LIMIT_KEY_PREFIX", "beester:ratelimit"),
			FailOpen:        rateLimitFailOpen,
			TrustProxy:      trustProxy,
		},
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func loadDotEnv() error {
	err := godotenv.Load()
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("load .env: %w", err)
}

func parseGRPCUpstreams(value string) ([]GRPCUpstream, error) {
	entries, err := parseNamedTargets(value, "gRPC upstream")
	if err != nil {
		return nil, err
	}

	upstreams := make([]GRPCUpstream, 0, len(entries))
	for _, entry := range entries {
		upstreams = append(upstreams, GRPCUpstream{Name: entry.name, Target: entry.target})
	}
	return upstreams, nil
}

func parseHTTPUpstreams(value string) ([]HTTPUpstream, error) {
	entries, err := parseNamedTargets(value, "HTTP upstream")
	if err != nil {
		return nil, err
	}

	upstreams := make([]HTTPUpstream, 0, len(entries))
	for _, entry := range entries {
		upstreams = append(upstreams, HTTPUpstream{Name: entry.name, Target: entry.target})
	}
	return upstreams, nil
}

type namedTarget struct {
	name   string
	target string
}

func parseNamedTargets(value string, kind string) ([]namedTarget, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	entries := strings.Split(value, ",")
	result := make([]namedTarget, 0, len(entries))
	names := make(map[string]struct{}, len(entries))

	for index, entry := range entries {
		entry = strings.TrimSpace(entry)
		name, target, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, fmt.Errorf("entry %d %q must use name=target format", index+1, entry)
		}

		name = strings.TrimSpace(name)
		target = strings.TrimSpace(target)
		if name == "" {
			return nil, fmt.Errorf("entry %d %s name is required", index+1, kind)
		}
		if target == "" {
			return nil, fmt.Errorf("%s %q target is required", kind, name)
		}
		if _, exists := names[name]; exists {
			return nil, fmt.Errorf("duplicate %s %q", kind, name)
		}

		names[name] = struct{}{}
		result = append(result, namedTarget{name: name, target: target})
	}

	return result, nil
}

func parseHTTPRoutes(value string) ([]HTTPRoute, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	entries := strings.Split(value, ",")
	routes := make([]HTTPRoute, 0, len(entries))
	keys := make(map[string]struct{}, len(entries))

	for index, entry := range entries {
		entry = strings.TrimSpace(entry)
		left, upstream, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, fmt.Errorf("entry %d %q must use METHOD:PATH=UPSTREAM format", index+1, entry)
		}

		method, pattern, ok := strings.Cut(left, ":")
		if !ok {
			return nil, fmt.Errorf("entry %d %q must use METHOD:PATH=UPSTREAM format", index+1, entry)
		}

		method = strings.ToUpper(strings.TrimSpace(method))
		pattern = strings.TrimSpace(pattern)
		upstream = strings.TrimSpace(upstream)
		if method == "" || pattern == "" || upstream == "" {
			return nil, fmt.Errorf("entry %d %q contains an empty method, path, or upstream", index+1, entry)
		}
		if !strings.HasPrefix(pattern, "/") {
			return nil, fmt.Errorf("HTTP route %s %q must start with /", method, pattern)
		}

		key := method + " " + pattern
		if _, exists := keys[key]; exists {
			return nil, fmt.Errorf("duplicate HTTP route %q", key)
		}
		keys[key] = struct{}{}
		routes = append(routes, HTTPRoute{Method: method, Pattern: pattern, Upstream: upstream})
	}

	return routes, nil
}

func parseGRPCRoutes(value string) ([]GRPCRoute, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	entries := strings.Split(value, ",")
	routes := make([]GRPCRoute, 0, len(entries))
	prefixes := make(map[string]struct{}, len(entries))

	for index, entry := range entries {
		entry = strings.TrimSpace(entry)
		prefix, upstream, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, fmt.Errorf("entry %d %q must use METHOD_PREFIX=UPSTREAM format", index+1, entry)
		}
		prefix = strings.TrimSpace(prefix)
		upstream = strings.TrimSpace(upstream)
		if prefix == "" || upstream == "" {
			return nil, fmt.Errorf("entry %d %q contains an empty prefix or upstream", index+1, entry)
		}
		if !strings.HasPrefix(prefix, "/") {
			return nil, fmt.Errorf("gRPC route prefix %q must start with /", prefix)
		}
		if _, exists := prefixes[prefix]; exists {
			return nil, fmt.Errorf("duplicate gRPC route prefix %q", prefix)
		}
		prefixes[prefix] = struct{}{}
		routes = append(routes, GRPCRoute{Prefix: prefix, Upstream: upstream})
	}

	return routes, nil
}

func (c Config) validate() error {
	if c.AppEnv == "" {
		return errors.New("APP_ENV is required")
	}
	if c.HTTPAddr == "" {
		return errors.New("HTTP_ADDR is required")
	}
	if c.MaxRequestBodyBytes < 0 {
		return errors.New("MAX_REQUEST_BODY_BYTES must be >= 0")
	}
	if c.UpstreamTimeout < 0 {
		return errors.New("UPSTREAM_REQUEST_TIMEOUT must be >= 0")
	}

	switch c.GRPCTransportSecurity {
	case GRPCTransportSecurityTLS, GRPCTransportSecurityInsecure:
	default:
		return fmt.Errorf("unsupported GRPC_TRANSPORT_SECURITY %q", c.GRPCTransportSecurity)
	}

	httpUpstreamNames := make(map[string]struct{}, len(c.HTTPUpstreams))
	for _, upstream := range c.HTTPUpstreams {
		httpUpstreamNames[upstream.Name] = struct{}{}
	}
	for _, route := range c.HTTPRoutes {
		if _, exists := httpUpstreamNames[route.Upstream]; !exists {
			return fmt.Errorf("HTTP route %s %q references unknown upstream %q", route.Method, route.Pattern, route.Upstream)
		}
	}

	grpcUpstreamNames := make(map[string]struct{}, len(c.GRPCUpstreams))
	for _, upstream := range c.GRPCUpstreams {
		grpcUpstreamNames[upstream.Name] = struct{}{}
	}
	for _, route := range c.GRPCRoutes {
		if _, exists := grpcUpstreamNames[route.Upstream]; !exists {
			return fmt.Errorf("gRPC route %q references unknown upstream %q", route.Prefix, route.Upstream)
		}
	}
	if len(c.GRPCRoutes) > 0 && c.GRPCAddr == "" {
		return errors.New("GRPC_ADDR is required when GRPC_ROUTES is configured")
	}

	if c.JWT.Enabled {
		if strings.TrimSpace(c.JWT.HMACSecret) == "" {
			return errors.New("JWT_HMAC_SECRET is required when JWT_ENABLED=true")
		}
		if strings.TrimSpace(c.JWT.UserIDHeader) == "" {
			return errors.New("JWT_USER_ID_HEADER is required when JWT_ENABLED=true")
		}
		if c.JWT.Leeway < 0 {
			return errors.New("JWT_LEEWAY must be >= 0")
		}
	}

	if c.RateLimit.Enabled {
		if strings.TrimSpace(c.RateLimit.RedisAddr) == "" {
			return errors.New("REDIS_ADDR is required when RATE_LIMIT_ENABLED=true")
		}
		if c.RateLimit.Capacity <= 0 {
			return errors.New("RATE_LIMIT_CAPACITY must be > 0")
		}
		if c.RateLimit.RefillPerSecond <= 0 {
			return errors.New("RATE_LIMIT_REFILL_PER_SECOND must be > 0")
		}
		if strings.TrimSpace(c.RateLimit.KeyPrefix) == "" {
			return errors.New("RATE_LIMIT_KEY_PREFIX is required when RATE_LIMIT_ENABLED=true")
		}
	}

	return nil
}

func envString(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func envList(name string, fallback []string) []string {
	value, exists := os.LookupEnv(name)
	if !exists || strings.TrimSpace(value) == "" {
		return append([]string(nil), fallback...)
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func envBool(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

func envInt(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

func envInt64(name string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

func envFloat64(name string, fallback float64) (float64, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}
