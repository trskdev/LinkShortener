package handler

import (
	"encoding/json"
	"net/http"
)

type apiError struct {
	Error string `json:"error"`
}

type StatusRecorder struct {
	http.ResponseWriter
	Status int
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{msg})
}
