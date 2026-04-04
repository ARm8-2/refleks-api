package status

import (
	"encoding/json"
	"net/http"
	"time"
)

// Handler serves the status endpoint.
type Handler struct {
	service *Service
	now     func() time.Time
}

// NewHandler creates an HTTP handler for service status.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
		now:     time.Now,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	response := h.service.Snapshot(h.now())

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
