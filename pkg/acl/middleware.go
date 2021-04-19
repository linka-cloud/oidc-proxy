package acl

import (
	"net/http"

	oidc_handlers "gitlab.bertha.cloud/partitio/lab/oidc-handlers"
)

func NewACLMiddleware(a *ACL) (*ACLMiddleware, error) {
	if a == nil {
		a = &ACL{}
	}
	a.Normalize()
	return &ACLMiddleware{acl: *a}, nil
}

func (m *ACLMiddleware) UpdateACL(a *ACL) {
	if a == nil {
		a = &ACL{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	a.Normalize()
	m.acl = *a
}

func (m *ACLMiddleware) EnforceFunc(next http.HandlerFunc) http.Handler {
	return m.Enforce(next)
}

func (m *ACLMiddleware) Enforce(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		endpoint, ok := m.acl.Endpoints.For(r.Host)
		if !ok {
			m.enforce(next, w, r, m.acl.DefaultPolicy)
			return
		}
		claims, ok := oidc_handlers.ClaimsFromContext(r.Context())
		if !ok {
			m.enforce(next, w, r, endpoint.DefaultPolicy)
			return
		}
		var found bool
		for _, v := range claims.Groups {
			g, ok := endpoint.Groups.For(v)
			if !ok {
				continue
			}
			found = true
			if g.Policy.Equals(RejectPolicy) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		if !found {
			m.enforce(next, w, r, endpoint.DefaultPolicy)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *ACLMiddleware) enforce(next http.Handler, w http.ResponseWriter, r *http.Request, policy Policy) {
	switch {
	case policy.Equals(AcceptPolicy):
		next.ServeHTTP(w, r)
	default:
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
}
