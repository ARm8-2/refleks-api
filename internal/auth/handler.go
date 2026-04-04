package auth

import (
	"encoding/json"
	"net/http"
)

// Handler serves auth-related HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates an auth HTTP handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// HandleSessionStub returns auth scaffold information.
func (h *Handler) HandleSessionStub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	response := h.service.SessionStub(r.Context())
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// HandleSteamLoginStub returns Steam login scaffold information.
func (h *Handler) HandleSteamLoginStub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	response := h.service.SteamLoginStub()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
