package api

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Detail string `json:"detail"`
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteError(w http.ResponseWriter, status int, detail string) {
	WriteJSON(w, status, ErrorResponse{Detail: detail})
}
