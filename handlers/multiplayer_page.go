package handlers

import (
	"html/template"
	"net/http"
)

type MultiplayerPageHandler struct {
}

func (h *MultiplayerPageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/multiplayer.html"))
	tmpl.Execute(w, nil)
}
