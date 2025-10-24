package config

import (
	"context"
	"errors"
)

var ErrConfigNotFoundContext = errors.New("config not found in context")

type contextKey struct{}

// ToContext adds a Config to the context.
func ToContext(ctx context.Context, cfg *Config) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKey{}, cfg)
}

// FromContext returns the Config from the context.
func FromContext(ctx context.Context) (*Config, error) {
	if ctx == nil {
		return nil, ErrConfigNotFoundContext
	}

	cfg, ok := ctx.Value(contextKey{}).(*Config)
	if !ok || cfg == nil {
		return nil, ErrConfigNotFoundContext
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// MustFromContext panics if config not in context (use sparingly).
func MustFromContext(ctx context.Context) *Config {
	cfg, err := FromContext(ctx)
	if err != nil {
		panic(err)
	}
	return cfg
}
