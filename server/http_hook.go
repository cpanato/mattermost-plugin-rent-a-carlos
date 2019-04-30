package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/plugin"
)

// ServeHTTP allows the plugin to implement the http.Handler interface. Requests destined for the
// /plugins/{id} path will be routed to the plugin.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" || strings.Compare(token, p.configuration.Token) != 0 {
		errorMessage := "Invalid or missing token"
		http.Error(w, errorMessage, http.StatusBadRequest)
		return
	}

	switch r.URL.Path {
	case "/api/complete":
		p.handleCompleteTask(w, r)
	case "/api/reject":
		p.handleRejectTask(w, r)
	case "/api/dialog":
		p.handleDialog(w, r)
	default:
		http.NotFound(w, r)
	}
}

func encodeEphermalMessage(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	payload := map[string]interface{}{
		"ephemeral_text": message,
	}

	json.NewEncoder(w).Encode(payload)
}
