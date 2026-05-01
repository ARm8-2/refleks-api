package players

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Handler serves player browsing endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a player handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// HandleList returns a paginated, searchable player list.
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

	resp, err := h.service.ListPlayers(r.Context(), ListRequest{
		Query:  strings.TrimSpace(q.Get("q")),
		Sort:   PlayerSort(strings.TrimSpace(q.Get("sort"))),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleGet returns the profile for one player by their Steam ID path value.
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	steamID := strings.TrimSpace(r.PathValue("steam_id"))
	if steamID == "" {
		writeError(w, http.StatusBadRequest, "steam_id is required")
		return
	}

	player, err := h.service.GetPlayer(r.Context(), steamID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "player not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, player)
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
