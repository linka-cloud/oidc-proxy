package proxy

import (
	"context"
	"net/url"

	oidc_handlers "go.linka.cloud/oidc-handlers"
)

type options struct {
	ctx            context.Context
	u              *url.URL
	oidcConfig     oidc_handlers.Config
	configPath     string
	address        string
	allowedOrigins []string
}

type Option func(o *options)

func WithContext(ctx context.Context) Option {
	return func(o *options) {
		o.ctx = ctx
	}
}

func WithBackend(backend *url.URL) Option {
	return func(o *options) {
		o.u = backend
	}
}

func WithOIDC(oidcConfig oidc_handlers.Config) Option {
	return func(o *options) {
		o.oidcConfig = oidcConfig
	}
}

func WithConfig(path string) Option {
	return func(o *options) {
		o.configPath = path
	}
}

func WithAddress(address string) Option {
	return func(o *options) {
		o.address = address
	}
}

func WithAllowedOrigins(origin ...string) Option {
	return func(o *options) {
		o.allowedOrigins = origin
	}
}
