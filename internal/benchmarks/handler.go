package benchmarks

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const benchmarkListCacheControl = "public, max-age=0, must-revalidate"

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

	body, etag, err := encodeJSONWithETag(resp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Cache-Control", benchmarkListCacheControl)
	w.Header().Set("ETag", etag)
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
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

func encodeJSONWithETag(payload any) ([]byte, string, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return nil, "", err
	}

	body := bytes.TrimRight(buf.Bytes(), "\n")
	sum := sha256.Sum256(body)
	etag := fmt.Sprintf("\"%x\"", sum)
	return body, etag, nil
}

func etagMatches(ifNoneMatch, currentETag string) bool {
	ifNoneMatch = strings.TrimSpace(ifNoneMatch)
	if ifNoneMatch == "" {
		return false
	}

	for _, candidate := range strings.Split(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if candidate == "*" {
			return true
		}
		if normalizeETag(candidate) == normalizeETag(currentETag) {
			return true
		}
	}

	return false
}

func normalizeETag(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "W/")
	raw = strings.TrimSpace(raw)
	return raw
}
