package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	toolkit_cli "go.linka.cloud/grpc-toolkit/cli"
	"go.linka.cloud/grpc-toolkit/logger"

	oidc_handlers "go.linka.cloud/oidc-handlers"
	"go.linka.cloud/oidc-proxy/pkg/proxy"
)

const allowedOriginsEnv = "ALLOWED_ORIGINS"

type OIDC struct {
	IssuerURL    string `name:"issuer-url" usage:"oidc issuer URL" env:"ISSUER_URL"`
	ClientID     string `name:"client-id" usage:"oidc client id" env:"CLIENT_ID"`
	ClientSecret string `name:"client-secret" usage:"oidc client secret" env:"CLIENT_SECRET"`
	CallbackURL  string `name:"callback-url" usage:"oidc callback URL" env:"CALLBACK_URL"`
}

type Cookie struct {
	CookieDomain       string `name:"cookie-domain" usage:"cookie domain" env:"COOKIE_DOMAIN"`
	IDTokenCookie      string `name:"id-token-cookie" usage:"the id token cookie name" env:"ID_TOKEN_COOKIE" default:"id_token"`
	RefreshTokenCookie string `name:"refresh-token-cookie" usage:"the refresh token cookie name" env:"REFRESH_TOKEN_COOKIE" default:"refresh_token"`
	AuthStateCookie    string `name:"auth-state-cookie" usage:"the auth state cookie name" env:"AUTH_STATE_COOKIE" default:"auth_state"`
	RedirectCookie     string `name:"redirect-cookie" usage:"the redirect cookie name" env:"REDIRECT_COOKIE" default:"redirect"`
	CookieSecure       bool   `name:"cookie-secure" usage:"whether the cookie is secure" env:"COOKIE_SECURE"`
	SessionKey         string `name:"session-key" usage:"the session key used to encrypt the session cookie" env:"SESSION_KEY"`
}

type MTLS struct {
	ClientCA   string `name:"client-ca" usage:"the client ca used to verify the backend cert" env:"CLIENT_CA"`
	ClientKey  string `name:"client-key" usage:"the client key used to authenticate to the backend with mTLS" env:"CLIENT_KEY"`
	ClientCert string `name:"client-cert" usage:"the client cert used to authenticate to the backend with mTLS" env:"CLIENT_CERT"`
}

type serveCmd struct {
	OIDC
	Cookie
	MTLS
	Address        string            `name:"address" usage:"listen address" env:"ADDRESS" default:":8888"`
	ConfigPath     string            `name:"config" usage:"acl config path" default:"config.yaml"`
	AllowedOrigins []string          `name:"allowed-origins" usage:"cors' allowed origins" env:"ALLOWED_ORIGINS"`
	BackendHeaders map[string]string `name:"backend-header" usage:"header to pass to backend (repeat key=value)"`
}

type runCfg struct {
	origins []string
	headers map[string]string
}

func (c *serveCmd) Run(cmd *cobra.Command, args []string) error {
	cfg, err := c.resolve()
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	log := logger.C(ctx).WithField("component", "oidc-proxy")
	u, err := parseBackend(args[0])
	if err != nil {
		return err
	}
	p, err := proxy.New(c.opts(ctx, u, cfg)...)
	if err != nil {
		log.WithError(err).Error("create proxy")
		return err
	}
	return p.Serve()
}

func (c *serveCmd) resolve() (*runCfg, error) {
	if c.IssuerURL == "" {
		return nil, fmt.Errorf("issuer-url is required")
	}
	if c.ClientID == "" {
		return nil, fmt.Errorf("client-id is required")
	}
	if c.ClientSecret == "" {
		return nil, fmt.Errorf("client-secret is required")
	}
	origins := c.allowedOrigins()
	if err := validateOrigins(origins); err != nil {
		return nil, err
	}
	headers, err := c.backendHeaders()
	if err != nil {
		return nil, err
	}
	return &runCfg{
		origins: origins,
		headers: headers,
	}, nil
}

func (c *serveCmd) opts(ctx context.Context, u *url.URL, cfg *runCfg) []proxy.Option {
	opts := []proxy.Option{
		proxy.WithContext(ctx),
		proxy.WithBackend(u),
		proxy.WithOIDC(c.oidcConfig()),
		proxy.WithConfig(c.ConfigPath),
		proxy.WithAddress(c.Address),
		proxy.WithAllowedOrigins(cfg.origins...),
		proxy.WithBackendHeaders(cfg.headers),
	}
	return append(opts, c.mtlsOpts()...)
}

func (c *serveCmd) mtlsOpts() []proxy.Option {
	var opts []proxy.Option
	if c.ClientCA != "" {
		opts = append(opts, proxy.WithClientCA(c.ClientCA))
	}
	if c.ClientKey != "" {
		opts = append(opts, proxy.WithClientKey(c.ClientKey))
	}
	if c.ClientCert != "" {
		opts = append(opts, proxy.WithClientCert(c.ClientCert))
	}
	return opts
}

func (c *serveCmd) oidcConfig() oidc_handlers.Config {
	return oidc_handlers.Config{
		IssuerURL:     c.IssuerURL,
		ClientID:      c.ClientID,
		ClientSecret:  c.ClientSecret,
		OauthCallback: c.CallbackURL,
		CookieConfig: oidc_handlers.CookieConfig{
			Domain:           c.CookieDomain,
			IDTokenName:      c.IDTokenCookie,
			RefreshTokenName: c.RefreshTokenCookie,
			AuthStateName:    c.AuthStateCookie,
			RedirectName:     c.RedirectCookie,
			Secure:           c.CookieSecure,
			Key:              c.SessionKey,
		},
	}
}

func (c *serveCmd) allowedOrigins() []string {
	if len(c.AllowedOrigins) > 0 {
		return c.AllowedOrigins
	}
	raw := os.Getenv(allowedOriginsEnv)
	if raw == "" {
		return nil
	}
	var origins []string
	for _, v := range splitCSV(raw) {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		origins = append(origins, v)
	}
	return origins
}

func splitCSV(raw string) []string {
	vals, err := csv.NewReader(strings.NewReader(raw)).Read()
	if err != nil {
		return strings.Split(raw, ",")
	}
	return vals
}

func (c *serveCmd) backendHeaders() (map[string]string, error) {
	headers := make(map[string]string, len(c.BackendHeaders))
	for k, v := range c.BackendHeaders {
		k = strings.TrimSpace(k)
		if k == "" {
			return nil, fmt.Errorf("backend-header: header key is required")
		}
		if strings.ContainsAny(k, "\r\n") {
			return nil, fmt.Errorf("backend-header: invalid header key %q", k)
		}
		if strings.ContainsAny(v, "\r\n") {
			return nil, fmt.Errorf("backend-header: invalid value for %q", k)
		}
		headers[http.CanonicalHeaderKey(k)] = strings.TrimSpace(v)
	}
	return headers, nil
}

func parseBackend(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse backend: %w", err)
	}
	if u.Scheme == "" {
		return nil, fmt.Errorf("parse backend: scheme is required")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("parse backend: unsupported scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("parse backend: host is required")
	}
	return u, nil
}

func validateOrigins(origins []string) error {
	for _, v := range origins {
		if v == "*" {
			return fmt.Errorf("allowed-origins: wildcard origin is not allowed")
		}
		u, err := url.Parse(v)
		if err != nil {
			return fmt.Errorf("allowed-origins: parse %q: %w", v, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("allowed-origins: unsupported scheme %q for %q", u.Scheme, v)
		}
		if u.Host == "" {
			return fmt.Errorf("allowed-origins: host is required for %q", v)
		}
		if u.Path != "" && u.Path != "/" {
			return fmt.Errorf("allowed-origins: path is not allowed for %q", v)
		}
		if u.RawQuery != "" {
			return fmt.Errorf("allowed-origins: query is not allowed for %q", v)
		}
		if u.Fragment != "" {
			return fmt.Errorf("allowed-origins: fragment is not allowed for %q", v)
		}
	}
	return nil
}

func main() {
	root := &cobra.Command{
		Use:   "oidc-proxy",
		Short: "An oidc Proxy",
	}
	root.AddCommand(toolkit_cli.Command(&serveCmd{}, &cobra.Command{
		Use:  "serve [backend]",
		Args: cobra.ExactArgs(1),
	}))
	toolkit_cli.Main(root)
}
