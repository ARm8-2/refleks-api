package benchmarks

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Handler serves benchmark-related endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a benchmark handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// HandleList returns benchmark definitions.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	includeInactive, err := optionalBoolQuery(r.URL.Query().Get("include_inactive"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "include_inactive must be true or false")
		return
	}

	resp, err := h.service.ListBenchmarks(r.Context(), ListRequest{
		Query:           strings.TrimSpace(r.URL.Query().Get("q")),
		IncludeInactive: includeInactive,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func optionalBoolQuery(raw string) (bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, nil
	}
	return strconv.ParseBool(raw)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
