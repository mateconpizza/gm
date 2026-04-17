package application

import (
	"context"
	"errors"
)

var ErrConfigNotFoundContext = errors.New("config not found in context")

type contextKey struct{}

// ToContext adds a Config to the context.
func ToContext(ctx context.Context, app *App) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKey{}, app)
}

// FromContext returns the Config from the context.
func FromContext(ctx context.Context) (*App, error) {
	if ctx == nil {
		return nil, ErrConfigNotFoundContext
	}

	app, ok := ctx.Value(contextKey{}).(*App)
	if !ok || app == nil {
		return nil, ErrConfigNotFoundContext
	}

	if err := app.Validate(); err != nil {
		return nil, err
	}

	return app, nil
}

// MustFromContext panics if config not in context (use sparingly).
func MustFromContext(ctx context.Context) *App {
	app, err := FromContext(ctx)
	if err != nil {
		panic(err)
	}
	return app
}
