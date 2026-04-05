package benchmarks

import (
	"encoding/json"
	"errors"
	"net/http"
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

	resp, err := h.service.ListBenchmarks(r.Context(), ListRequest{
		Query: strings.TrimSpace(r.URL.Query().Get("q")),
		View:  parseListView(r.URL.Query().Get("view")),
	})
	if err != nil {
		if errors.Is(err, ErrInvalidView) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func parseListView(raw string) ListView {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ListViewFull
	}
	return ListView(raw)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}
