package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/sirupsen/logrus"
	oidc_handlers "gitlab.bertha.cloud/partitio/lab/oidc-handlers"
	acl2 "gitlab.bertha.cloud/partitio/lab/oidc-proxy/pkg/acl"
	"gitlab.bertha.cloud/partitio/lab/oidc-proxy/pkg/config"
)

type proxy struct {
	mux     *http.ServeMux
	address string
}

func New(ctx context.Context, u *url.URL, oidcConfig oidc_handlers.Config, configPath string, address string) (*proxy, error) {
	prox := httputil.NewSingleHostReverseProxy(u)

	oidcConfig.Logger = logrus.New()
	oidc, err := oidc_handlers.New(ctx, oidcConfig)
	if err != nil {
		return nil, err
	}
	cURL, err := url.Parse(oidcConfig.OauthCallback)
	if err != nil {
		return nil, fmt.Errorf("parse callback url: %w", err)
	}
	conf, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	acl, err := acl2.NewACLMiddleware(conf.ACL())
	if err != nil {
		return nil, err
	}
	aclChan := conf.Watch()
	go func() {
		acl.UpdateACL(<-aclChan)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/oidc/auth", oidc.RedirectHandler)
	mux.HandleFunc(cURL.Path, oidc.CallbackHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tk, err := oidc.Refresh(w, r)
		if err != nil {
			logrus.Error(err)
			oidc.SetRedirectCookie(w, "/")
			http.Redirect(w, r, "/oidc/auth", http.StatusSeeOther)
			return
		}
		if c, err := r.Cookie(oidcConfig.CookieConfig.IDTokenName); tk == "" && err == nil {
			tk = c.Value
		}
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tk))
		acl.EnforceFunc(prox.ServeHTTP).ServeHTTP(w, r)
	})
	return &proxy{mux: mux, address: address}, nil
}

func (p *proxy) Run() error {
	return http.ListenAndServe(p.address, p.mux)
}
