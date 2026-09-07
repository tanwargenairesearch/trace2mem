package server

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed console.html console.js console.css guide.html
var consoleFiles embed.FS
var pageTemplate = template.Must(template.ParseFS(consoleFiles, "console.html"))

func (s *Server) console(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/guide" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		b, _ := consoleFiles.ReadFile("guide.html")
		w.Write(b)
		return
	}
	if r.URL.Path == "/console.js" || r.URL.Path == "/console.css" {
		if r.URL.Path == "/console.css" {
			w.Header().Set("Content-Type", "text/css")
		} else {
			w.Header().Set("Content-Type", "text/javascript")
		}
		b, _ := consoleFiles.ReadFile(r.URL.Path[1:])
		w.Write(b)
		return
	}
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	_, e := s.cookiePrincipal(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	pageTemplate.Execute(w, map[string]bool{"LoggedIn": e == nil, "OIDC": s.OIDC != nil, "Scripted": s.Config.Scripted})
}
