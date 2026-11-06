package httpapi

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func WriteError(w http.ResponseWriter, status int, code, message string, extra map[string]any) {
	body := map[string]any{
		"code":    code,
		"message": message,
	}
	for k, v := range extra {
		body[k] = v
	}

	WriteJSON(w, status, map[string]any{"error": body})
}
