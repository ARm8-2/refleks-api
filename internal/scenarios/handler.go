package scenarios

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Handler serves scenario browsing endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a scenario handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// HandleList returns a paginated, searchable scenario list.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit, err := optionalInt(q.Get("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "limit must be an integer")
		return
	}
	offset, err := optionalInt(q.Get("offset"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "offset must be an integer")
		return
	}

	resp, err := h.service.ListScenarios(r.Context(), ListRequest{
		Query:  strings.TrimSpace(q.Get("q")),
		Sort:   ScenarioSort(strings.TrimSpace(q.Get("sort"))),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleGet returns full metadata for one scenario by its database ID.
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	rawID := strings.TrimSpace(r.PathValue("id"))
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id must be a positive integer")
		return
	}

	detail, err := h.service.GetScenario(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "scenario not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func optionalInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}
