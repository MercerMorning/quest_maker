package handlers

import (
	"html/template"
	"net/http"
)

type RootHandler struct {
}

func (h *RootHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/root.html"))
	tmpl.Execute(w, nil)
}
