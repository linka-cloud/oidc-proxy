package main

import (
	"context"
	"net/url"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	proxy "go.linka.cloud/oidc-proxy/pkg/proxy"

	oidc_handlers "go.linka.cloud/oidc-handlers"
)

func main() {
	oidcConfig := oidc_handlers.Config{}
	configPath := "config.yaml"
	address := ":8888"
	allowedOrigins := &cli.StringSlice{}

	app := cli.NewApp()
	app.Name = "oidc-proxy"
	app.Usage = "An OIDC Proxy"
	app.Commands = []*cli.Command{
		{
			Name:      "serve",
			Usage:     "serve [backend]",
			ArgsUsage: "backend: the backend to be proxied",
			Flags: append(app.Flags,
				&cli.StringFlag{
					Name:        "issuer-url",
					Destination: &oidcConfig.IssuerURL,
					EnvVars:     []string{"ISSUER_URL"},
					Required:    true,
				},
				&cli.StringFlag{
					Name:        "client-id",
					Destination: &oidcConfig.ClientID,
					EnvVars:     []string{"CLIENT_ID"},
					Required:    true,
				},
				&cli.StringFlag{
					Name:        "client-secret",
					Destination: &oidcConfig.ClientSecret,
					EnvVars:     []string{"CLIENT_SECRET"},
					Required:    true,
				},
				&cli.StringFlag{
					Name:        "callback-url",
					Destination: &oidcConfig.OauthCallback,
					EnvVars:     []string{"CALLBACK_URL"},
				},
				&cli.StringFlag{
					Name:        "address",
					Destination: &address,
					EnvVars:     []string{"ADDRESS"},
					Value:       ":8888",
				},
				&cli.StringFlag{
					Name:        "cookie-domain",
					Destination: &oidcConfig.CookieConfig.Domain,
					EnvVars:     []string{"COOKIE_DOMAIN"},
				},
				&cli.StringFlag{
					Name:        "id-token-cookie",
					Destination: &oidcConfig.CookieConfig.IDTokenName,
					EnvVars:     []string{"ID_TOKEN_COOKIE"},
					Usage:       "the id token cookie name",
					Value:       "id_token",
				},
				&cli.StringFlag{
					Name:        "refresh-token-cookie",
					Destination: &oidcConfig.CookieConfig.RefreshTokenName,
					EnvVars:     []string{"REFRESH_TOKEN_COOKIE"},
					Usage:       "the refresh token cookie name",
					Value:       "refresh_token",
				},
				&cli.StringFlag{
					Name:        "auth-state-cookie",
					Destination: &oidcConfig.CookieConfig.AuthStateName,
					EnvVars:     []string{"AUTH_STATE_COOKIE"},
					Usage:       "the auth state cookie name",
					Value:       "auth_state",
				},
				&cli.StringFlag{
					Name:        "redirect-cookie",
					Destination: &oidcConfig.CookieConfig.RedirectName,
					EnvVars:     []string{"REDIRECT_COOKIE"},
					Usage:       "the redirect cookie name",
					Value:       "redirect",
				},
				&cli.BoolFlag{
					Name:        "cookie-secure",
					Destination: &oidcConfig.CookieConfig.Secure,
					EnvVars:     []string{"COOKIE_SECURE"},
					Usage:       "whether the cookie is secure",
				},
				&cli.StringFlag{
					Name:        "session-key",
					Usage:       "the session key used to encrypt the session cookie.",
					Destination: &oidcConfig.CookieConfig.Key,
					EnvVars:     []string{"SESSION_KEY"},
				},
				&cli.StringFlag{
					Name:        "config",
					Destination: &configPath,
				},
				&cli.StringSliceFlag{
					Name:        "allowed-origins",
					Usage:       "cors' allowed origins",
					Destination: allowedOrigins,
					EnvVars:     []string{"ALLOWED_ORIGINS"},
				},
			),
			Action: func(c *cli.Context) error {
				if c.Args().Len() == 0 {
					logrus.Fatal("expected proxy backend as argument")
				}

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				u, err := url.Parse(c.Args().First())
				if err != nil {
					logrus.Fatalf("failed to parse backend: %v", err)
				}
				if u.Scheme == "" {
					u.Scheme = "http"
				}
				proxy, err := proxy.New(
					proxy.WithContext(ctx),
					proxy.WithBackend(u),
					proxy.WithOIDC(oidcConfig),
					proxy.WithConfig(configPath),
					proxy.WithAddress(address),
					proxy.WithAllowedOrigins(allowedOrigins.Value()...),
				)
				if err != nil {
					logrus.Fatal(err)
				}
				return proxy.Serve()
			},
		},
	}
	app.Run(os.Args)
}
