package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	oidc_handlers "gitlab.bertha.cloud/partitio/lab/oidc-handlers"
)

func main() {
	conf := oidc_handlers.Config{}
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
					Destination: &conf.IssuerURL,
					EnvVars:     []string{"ISSUER_URL"},
					Required:    true,
				},
				&cli.StringFlag{
					Name:        "client-id",
					Destination: &conf.ClientID,
					EnvVars:     []string{"CLIENT_ID"},
					Required:    true,
				},
				&cli.StringFlag{
					Name:        "client-secret",
					Destination: &conf.ClientSecret,
					EnvVars:     []string{"CLIENT_SECRET"},
					Required:    true,
				},
				&cli.StringFlag{
					Name: "callback-url",
					Destination: &conf.OauthCallback,
					EnvVars: []string{"CALLBACK_URL"},
				},
				&cli.StringFlag{
					Name:        "address",
					Destination: &address,
					EnvVars:     []string{"ADDRESS"},
					Value:       ":8888",
				},
				&cli.StringFlag{
					Name:        "web-url",
					Destination: &conf.CookieConfig.Domain,
					EnvVars:     []string{"WEB_URL"},
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

				prox := httputil.NewSingleHostReverseProxy(u)

				conf.Logger = logrus.New()
				oidc, err := oidc_handlers.New(ctx, conf)
				if err != nil {
					logrus.Fatal(err)
				}
				cURL, err := url.Parse(conf.OauthCallback)
				if err != nil {
				    logrus.Fatalf("parse callback url: %v", err)
				}
				http.HandleFunc("/oidc/auth", oidc.RedirectHandler)
				http.HandleFunc(cURL.Path, oidc.CallbackHandler)
				http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					tk, err := oidc.Refresh(w, r)
					if err != nil {
						logrus.Error(err)
						oidc.SetRedirectCookie(w, "/")
						http.Redirect(w, r, "/oidc/auth", http.StatusSeeOther)
						return
					}
					if c, err := r.Cookie(conf.CookieConfig.IDTokenName); tk == "" && err == nil {
						tk = c.Value
					}
					r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tk))
					prox.ServeHTTP(w, r)
				})
				if err := http.ListenAndServe(address, nil); err != nil {
					logrus.Fatal(err)
				}
				return nil
			},
		},
	}
	app.Run(os.Args)
}
