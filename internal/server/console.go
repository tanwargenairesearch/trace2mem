package server

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed console.html console.js
var consoleFiles embed.FS
var pageTemplate = template.Must(template.ParseFS(consoleFiles, "console.html"))

func (s *Server) console(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/console.js" {
		w.Header().Set("Content-Type", "text/javascript")
		b, _ := consoleFiles.ReadFile("console.js")
		w.Write(b)
		return
	}
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	_, e := s.cookiePrincipal(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	pageTemplate.Execute(w, map[string]bool{"LoggedIn": e == nil, "OIDC": s.OIDC != nil})
}
