package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"

	"github.com/rs/cors"
	"go.linka.cloud/grpc-toolkit/logger"
	oidch "go.linka.cloud/oidc-handlers"

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

	mw, err := opts.oidcConfig.WebMiddleware(opts.ctx, oidch.WebMiddlewareConfig{
		LoginPath:  "/oidc/auth",
		LogoutPath: "/oidc/logout",
	})
	if err != nil {
		return nil, err
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
			select {
			case a, ok := <-aclChan:
				if !ok {
					return
				}
				acl.UpdateACL(a)
			case <-opts.ctx.Done():
				return
			}
		}
	}()

	h := mw(acl.Enforce(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tk, ok := oidch.RawIDTokenFromContext(r.Context())
		if !ok || tk == "" {
			logger.C(r.Context()).WithField("component", "proxy").Error("raw id token missing from context")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tk))
		setBackendHeaders(r, opts.backendHeaders)
		prox.ServeHTTP(w, r)
	})))
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
	return &proxy{mux: cors.Handler(h), opts: opts}, nil
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

func setBackendHeaders(r *http.Request, headers map[string]string) {
	for k, v := range headers {
		r.Header.Set(k, v)
	}
}
