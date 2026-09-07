package server

import (
	"encoding/base64"
	"github.com/trace2mem/trace2mem/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginOriginAndReferrerPolicy(t *testing.T) {
	vault, e := config.NewVault(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if e != nil {
		t.Fatal(e)
	}
	s := &Server{Config: config.Config{PublicURL: "http://localhost:18787/", BootstrapToken: "test-token"}, Vault: vault}
	page := httptest.NewRecorder()
	s.Handler().ServeHTTP(page, httptest.NewRequest("GET", "http://localhost:18787/", nil))
	if page.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Fatal("form submissions can lose their origin")
	}
	for _, tc := range []struct {
		origin string
		status int
	}{{"http://localhost:18787", 303}, {"https://foreign.example", 403}, {"null", 403}} {
		r := httptest.NewRequest("POST", "http://localhost:18787/login", strings.NewReader("token=test-token"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		s.login(w, r)
		if w.Code != tc.status {
			t.Fatalf("origin %q: status %d", tc.origin, w.Code)
		}
		if tc.status == http.StatusSeeOther && len(w.Result().Cookies()) != 1 {
			t.Fatal("login did not set session")
		}
	}
}
