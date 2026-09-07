package server

import (
	"connectrpc.com/connect"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/trace2mem/trace2mem/gen/trace2mem/v1/trace2memv1connect"
	"github.com/trace2mem/trace2mem/internal/blob"
	"github.com/trace2mem/trace2mem/internal/config"
	"github.com/trace2mem/trace2mem/internal/domain"
	"github.com/trace2mem/trace2mem/internal/dream"
	"github.com/trace2mem/trace2mem/internal/store"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	Store  *store.Store
	Blob   blob.Store
	Vault  config.Vault
	Config config.Config
	Engine *dream.Engine
	OIDC   *oidc.Provider
}
type principalKey struct{}

func principal(ctx context.Context) domain.Principal {
	p, _ := ctx.Value(principalKey{}).(domain.Principal)
	return p
}
func rpcerr(err error) error {
	if err == nil {
		return nil
	}
	code := connect.CodeInternal
	switch {
	case errors.Is(err, domain.ErrNotFound):
		code = connect.CodeNotFound
	case errors.Is(err, domain.ErrForbidden):
		code = connect.CodePermissionDenied
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrLease):
		code = connect.CodeFailedPrecondition
	case errors.Is(err, context.Canceled):
		code = connect.CodeCanceled
	case errors.Is(err, context.DeadlineExceeded):
		code = connect.CodeDeadlineExceeded
	}
	if code == connect.CodeInternal {
		slog.Error("request failed", "error", err)
		return connect.NewError(code, errors.New("internal request failure"))
	}
	return connect.NewError(code, err)
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	opts := []connect.HandlerOption{connect.WithReadMaxBytes(12 << 20), connect.WithSendMaxBytes(16 << 20)}
	p, h := trace2memv1connect.NewIngestionServiceHandler(s, opts...)
	mux.Handle(p, s.auth(h))
	p, h = trace2memv1connect.NewMemoryServiceHandler(s, opts...)
	mux.Handle(p, s.auth(h))
	mux.Handle("/mcp", s.auth(s.mcp()))
	mux.Handle("/api/", s.auth(http.HandlerFunc(s.management)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if e := s.Store.DB.Ping(ctx); e != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/login", s.login)
	mux.HandleFunc("/oauth/callback", s.callback)
	mux.HandleFunc("/.well-known/oauth-protected-resource", func(w http.ResponseWriter, r *http.Request) {
		if s.Config.OIDCIssuer == "" {
			http.NotFound(w, r)
			return
		}
		respond(w, map[string]any{"resource": s.Config.PublicURL + "/mcp", "authorization_servers": []string{s.Config.OIDCIssuer}, "scopes_supported": []string{"read", "write"}})
	})
	mux.HandleFunc("/", s.console)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == r.Header.Get("Authorization") && token != "" {
			http.Error(w, "bearer authentication required", 401)
			return
		}
		p, e := s.authenticate(r.Context(), token)
		if token == "" {
			p, e = s.cookiePrincipal(r)
		}
		if e != nil {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata="%s/.well-known/oauth-protected-resource"`, s.Config.PublicURL))
			http.Error(w, "authentication required", 401)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" && origin != strings.TrimRight(s.Config.PublicURL, "/") {
				http.Error(w, "origin rejected", 403)
				return
			}
			if token == "" && r.Header.Get("X-Trace2Mem-CSRF") != "1" {
				http.Error(w, "CSRF header required", 403)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
}
func (s *Server) authenticate(ctx context.Context, token string) (domain.Principal, error) {
	if token == "" {
		return domain.Principal{}, domain.ErrForbidden
	}
	if s.Config.BootstrapToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(s.Config.BootstrapToken)) == 1 {
		return domain.Principal{Tenant: "default", Subject: "admin", Admin: true, Scopes: map[string]bool{"read": true, "write": true}}, nil
	}
	p, e := s.Store.Token(ctx, token)
	if e == nil {
		return p, nil
	}
	if !errors.Is(e, domain.ErrForbidden) {
		return p, e
	}
	if s.OIDC != nil && s.Config.OAuthAudience != "" {
		v := s.OIDC.Verifier(&oidc.Config{ClientID: s.Config.OAuthAudience})
		id, e := v.Verify(ctx, token)
		if e != nil {
			return p, domain.ErrForbidden
		}
		var claims struct {
			Scope string `json:"scope"`
		}
		if e = id.Claims(&claims); e != nil {
			return p, e
		}
		p = domain.Principal{Tenant: "default", Subject: id.Subject, Scopes: map[string]bool{}}
		for _, scope := range strings.Fields(claims.Scope) {
			if scope == "read" || scope == "write" {
				p.Scopes[scope] = true
			}
		}
		return p, nil
	}
	return p, domain.ErrForbidden
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if e := json.NewEncoder(w).Encode(v); e != nil {
		slog.Error("response encoding failed", "error", e)
	}
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		http.Error(w, "invalid JSON request", 400)
		return false
	}
	return true
}
func failure(w http.ResponseWriter, e error) {
	status := 500
	message := "request failed"
	switch {
	case errors.Is(e, domain.ErrForbidden):
		status = 403
		message = e.Error()
	case errors.Is(e, domain.ErrNotFound):
		status = 404
		message = e.Error()
	case errors.Is(e, domain.ErrConflict):
		status = 409
		message = e.Error()
	}
	if status == 500 {
		slog.Error("management request failed", "error", e)
	}
	http.Error(w, message, status)
}
