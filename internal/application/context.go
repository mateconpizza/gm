package application

import (
	"context"
	"errors"
)

var (
	ErrAppNotFoundContext = errors.New("app: not found in context")
	ErrAppNoContext       = errors.New("app: context is nil")
)

type contextKey struct{}

// ToContext adds a App to the context.
func ToContext(ctx context.Context, app *App) (context.Context, error) {
	if ctx == nil {
		return nil, ErrAppNoContext
	}
	return context.WithValue(ctx, contextKey{}, app), nil
}

// FromContext returns the App from the context.
func FromContext(ctx context.Context) (*App, error) {
	if ctx == nil {
		return nil, ErrAppNoContext
	}

	app, ok := ctx.Value(contextKey{}).(*App)
	if !ok || app == nil {
		return nil, ErrAppNotFoundContext
	}

	if err := app.Validate(); err != nil {
		return nil, err
	}

	return app, nil
}
