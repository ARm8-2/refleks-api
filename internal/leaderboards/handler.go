package leaderboards

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Handler serves leaderboard endpoints.
type Handler struct {
	service *Service
}

// NewHandler constructs a leaderboard handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// HandleScenario returns one scenario leaderboard page.
func (h *Handler) HandleScenario(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	scenarioID, err := optionalInt64(r.URL.Query().Get("scenario_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "scenario_id must be an integer")
		return
	}
	limit, err := optionalInt(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "limit must be an integer")
		return
	}
	offset, err := optionalInt(r.URL.Query().Get("offset"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "offset must be an integer")
		return
	}

	resp, err := h.service.ScenarioLeaderboard(r.Context(), ScenarioRequest{
		ScenarioID:   scenarioID,
		ScenarioName: strings.TrimSpace(r.URL.Query().Get("scenario")),
		Limit:        limit,
		Offset:       offset,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleBenchmarkDifficulty returns one benchmark difficulty leaderboard page.
func (h *Handler) HandleBenchmarkDifficulty(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	difficultyID, err := optionalInt64(r.URL.Query().Get("difficulty_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "difficulty_id must be an integer")
		return
	}
	kovaaksID, err := optionalInt64(r.URL.Query().Get("kovaaks_benchmark_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "kovaaks_benchmark_id must be an integer")
		return
	}
	limit, err := optionalInt(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "limit must be an integer")
		return
	}
	offset, err := optionalInt(r.URL.Query().Get("offset"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "offset must be an integer")
		return
	}

	resp, err := h.service.BenchmarkDifficultyLeaderboard(r.Context(), BenchmarkDifficultyRequest{
		DifficultyID:       difficultyID,
		KovaaksBenchmarkID: kovaaksID,
		Limit:              limit,
		Offset:             offset,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidQuery):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "leaderboard target not found")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func optionalInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

func optionalInt64(raw string) (*int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
