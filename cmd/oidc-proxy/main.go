package main

import (
	"context"
	"encoding/csv"
	"fmt"
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
	Address        string   `name:"address" usage:"listen address" env:"ADDRESS" default:":8888"`
	ConfigPath     string   `name:"config" usage:"acl config path" default:"config.yaml"`
	AllowedOrigins []string `name:"allowed-origins" usage:"cors' allowed origins" env:"ALLOWED_ORIGINS"`
}

func (c *serveCmd) Run(cmd *cobra.Command, args []string) error {
	if err := c.validate(); err != nil {
		return err
	}

	ctx := cmd.Context()
	log := logger.C(ctx).WithField("component", "oidc-proxy")
	u, err := parseBackend(args[0])
	if err != nil {
		return err
	}
	p, err := proxy.New(c.opts(ctx, u)...)
	if err != nil {
		log.WithError(err).Error("create proxy")
		return err
	}
	return p.Serve()
}

func (c *serveCmd) validate() error {
	switch {
	case c.IssuerURL == "":
		return fmt.Errorf("issuer-url is required")
	case c.ClientID == "":
		return fmt.Errorf("client-id is required")
	case c.ClientSecret == "":
		return fmt.Errorf("client-secret is required")
	default:
		return nil
	}
}

func (c *serveCmd) opts(ctx context.Context, u *url.URL) []proxy.Option {
	opts := []proxy.Option{
		proxy.WithContext(ctx),
		proxy.WithBackend(u),
		proxy.WithOIDC(c.oidcHandlerConfig()),
		proxy.WithConfig(c.ConfigPath),
		proxy.WithAddress(c.Address),
		proxy.WithAllowedOrigins(c.allowedOrigins()...),
	}
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

func (c *serveCmd) oidcHandlerConfig() oidc_handlers.Config {
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
	vals, err := csv.NewReader(strings.NewReader(raw)).Read()
	if err != nil {
		vals = strings.Split(raw, ",")
	}
	var origins []string
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		origins = append(origins, v)
	}
	return origins
}

func parseBackend(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse backend: %w", err)
	}
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	return u, nil
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
