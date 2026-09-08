package server

import (
	"encoding/base64"
	"encoding/json"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/tanwargenairesearch/trace2mem/internal/domain"
	"github.com/tanwargenairesearch/trace2mem/internal/store"
	"golang.org/x/oauth2"
	"net/http"
	"strings"
	"time"
)

type session struct {
	Principal domain.Principal `json:"principal"`
	Expires   int64            `json:"expires"`
}

func (s *Server) cookiePrincipal(r *http.Request) (domain.Principal, error) {
	c, e := r.Cookie("trace2mem_session")
	if e != nil {
		return domain.Principal{}, domain.ErrForbidden
	}
	b, e := base64.RawURLEncoding.DecodeString(c.Value)
	if e != nil {
		return domain.Principal{}, domain.ErrForbidden
	}
	b, e = s.Vault.Open(r.Context(), b, []byte("browser-session"))
	if e != nil {
		return domain.Principal{}, domain.ErrForbidden
	}
	var v session
	if json.Unmarshal(b, &v) != nil || v.Expires < time.Now().Unix() {
		return domain.Principal{}, domain.ErrForbidden
	}
	return v.Principal, nil
}
func (s *Server) setSession(w http.ResponseWriter, r *http.Request, p domain.Principal) error {
	b, e := json.Marshal(session{p, time.Now().Add(8 * time.Hour).Unix()})
	if e != nil {
		return e
	}
	b, e = s.Vault.Seal(r.Context(), b, []byte("browser-session"))
	if e != nil {
		return e
	}
	http.SetCookie(w, &http.Cookie{Name: "trace2mem_session", Value: base64.RawURLEncoding.EncodeToString(b), Path: "/", HttpOnly: true, Secure: strings.HasPrefix(s.Config.PublicURL, "https:"), SameSite: http.SameSiteLaxMode, MaxAge: 28800})
	return nil
}
func (s *Server) oauth() oauth2.Config {
	return oauth2.Config{ClientID: s.Config.OIDCClient, ClientSecret: s.Config.OIDCSecret, RedirectURL: s.Config.PublicURL + "/oauth/callback", Endpoint: s.OIDC.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile"}}
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if origin := r.Header.Get("Origin"); origin != "" && origin != strings.TrimRight(s.Config.PublicURL, "/") {
			http.Error(w, "origin rejected", 403)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if e := r.ParseForm(); e != nil {
			http.Error(w, "invalid form", 400)
			return
		}
		p, e := s.authenticate(r.Context(), r.FormValue("token"))
		if e != nil {
			http.Error(w, "invalid token", 401)
			return
		}
		if e = s.setSession(w, r, p); e != nil {
			failure(w, e)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if s.OIDC == nil {
		http.Redirect(w, r, "/", 303)
		return
	}
	state := store.ID()
	verifier := oauth2.GenerateVerifier()
	b, _ := json.Marshal(map[string]string{"state": state, "verifier": verifier})
	b, e := s.Vault.Seal(r.Context(), b, []byte("oauth-state"))
	if e != nil {
		failure(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "trace2mem_oauth", Value: base64.RawURLEncoding.EncodeToString(b), Path: "/oauth/callback", HttpOnly: true, Secure: strings.HasPrefix(s.Config.PublicURL, "https:"), SameSite: http.SameSiteLaxMode, MaxAge: 300})
	c := s.oauth()
	http.Redirect(w, r, c.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oidc.Nonce(state)), 302)
}
func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	if s.OIDC == nil {
		http.NotFound(w, r)
		return
	}
	cookie, e := r.Cookie("trace2mem_oauth")
	if e != nil {
		http.Error(w, "missing login state", 400)
		return
	}
	b, e := base64.RawURLEncoding.DecodeString(cookie.Value)
	if e != nil {
		http.Error(w, "invalid login state", 400)
		return
	}
	b, e = s.Vault.Open(r.Context(), b, []byte("oauth-state"))
	var state map[string]string
	if e != nil || json.Unmarshal(b, &state) != nil || state["state"] != r.URL.Query().Get("state") {
		http.Error(w, "invalid login state", 400)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "trace2mem_oauth", Path: "/oauth/callback", MaxAge: -1})
	c := s.oauth()
	token, e := c.Exchange(r.Context(), r.URL.Query().Get("code"), oauth2.VerifierOption(state["verifier"]))
	if e != nil {
		http.Error(w, "identity exchange failed", 401)
		return
	}
	raw, _ := token.Extra("id_token").(string)
	id, e := s.OIDC.Verifier(&oidc.Config{ClientID: s.Config.OIDCClient}).Verify(r.Context(), raw)
	if e != nil || id.Nonce != state["state"] {
		http.Error(w, "identity verification failed", 401)
		return
	}
	p := domain.Principal{Tenant: "default", Subject: domain.Hash([]byte(id.Issuer + "\x00" + id.Subject)), Scopes: map[string]bool{"read": true, "ingest": true, "manage": true}}
	if e = s.setSession(w, r, p); e != nil {
		failure(w, e)
		return
	}
	http.Redirect(w, r, "/", 303)
}
