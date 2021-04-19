package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
	oidc_handlers "gitlab.bertha.cloud/partitio/lab/oidc-handlers"
	acl2 "gitlab.bertha.cloud/partitio/lab/oidc-proxy/pkg/acl"
	"gitlab.bertha.cloud/partitio/lab/oidc-proxy/pkg/config"
)

type proxy struct {
	mux  http.Handler
	opts *options
}

func New(opt ...Option) (*proxy, error) {
	opts := &options{}
	for _, v := range opt {
		v(opts)
	}
	if opts.ctx == nil {
		opts.ctx = context.Background()
	}
	if opts.address == "" {
		opts.address = ":8888"
	}
	prox := httputil.NewSingleHostReverseProxy(opts.u)

	opts.oidcConfig.Logger = logrus.New()
	oidc, err := oidc_handlers.New(opts.ctx, opts.oidcConfig)
	if err != nil {
		return nil, err
	}
	cURL, err := url.Parse(opts.oidcConfig.OauthCallback)
	if err != nil {
		return nil, fmt.Errorf("parse callback url: %w", err)
	}
	conf, err := config.Load(opts.configPath)
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
		if c, err := r.Cookie(opts.oidcConfig.CookieConfig.IDTokenName); tk == "" && err == nil {
			tk = c.Value
		}
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tk))
		acl.EnforceFunc(prox.ServeHTTP).ServeHTTP(w, r)
	})
	cors := cors.New(cors.Options{
		AllowedOrigins: opts.allowedOrigins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodHead,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})
	return &proxy{mux: cors.Handler(mux), opts: opts}, nil
}

func (p *proxy) Run() error {
	return http.ListenAndServe(p.opts.address, p.mux)
}
