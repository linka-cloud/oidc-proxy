package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/rs/cors"
	"github.com/sirupsen/logrus"

	acl2 "go.linka.cloud/oidc-proxy/pkg/acl"
	"go.linka.cloud/oidc-proxy/pkg/config"
)

type Proxy interface {
	Serve() error
	ServeTLS(certFile, keyFile string) error
	Handler() http.Handler
}

type proxy struct {
	mux  http.Handler
	opts *options
}

func New(opt ...Option) (Proxy, error) {
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

	if opts.clientCA != "" && opts.clientKey != "" && opts.clientCert != "" {
		pem, err := os.ReadFile(opts.clientCA)
		if err != nil {
			return nil, fmt.Errorf("read client ca: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(pem) {
			return nil, errors.New("failed to setup client ca")
		}
		cert, err := tls.LoadX509KeyPair(opts.clientCert, opts.clientKey)
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		prox.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            caCertPool,
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true,
			},
		}
	}

	opts.oidcConfig.Logger = logrus.New()
	oidc, err := opts.oidcConfig.WebHandler(opts.ctx)
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
		for {
			acl.UpdateACL(<-aclChan)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/oidc/auth", oidc.RedirectHandler)
	mux.HandleFunc("/oidc/logout", oidc.LogoutHandler)
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

func (p *proxy) Serve() error {
	return http.ListenAndServe(p.opts.address, p.mux)
}

func (p *proxy) ServeTLS(certFile, keyFile string) error {
	return http.ListenAndServeTLS(p.opts.address, certFile, keyFile, p.mux)
}

func (p *proxy) Handler() http.Handler {
	return p.mux
}
