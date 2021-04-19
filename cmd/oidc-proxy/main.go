package main

import (
	"context"
	"net/url"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	proxy "gitlab.bertha.cloud/partitio/lab/oidc-proxy/pkg/proxy"

	oidc_handlers "gitlab.bertha.cloud/partitio/lab/oidc-handlers"
)

func main() {
	oidcConfig := oidc_handlers.Config{}
	configPath := "config.yaml"
	address := ":8888"
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
					Name: "config",
					Destination: &configPath,
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
				proxy, err := proxy.New(ctx, u, oidcConfig, configPath, address)
				if err != nil {
					logrus.Fatal(err)
				}
				return proxy.Run()
			},
		},
	}
	app.Run(os.Args)
}
