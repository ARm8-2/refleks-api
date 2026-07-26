package runs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const multipartOverheadBytes = int64(1 << 20)

// HandlerConfig controls request limits for sync endpoints.
type HandlerConfig struct {
	MaxSingleFileBytes int64
	MaxBulkFileCount   int
	MaxBulkTotalBytes  int64
	MaxMissingHashes   int
}

// DefaultHandlerConfig returns safe operational defaults.
func DefaultHandlerConfig() HandlerConfig {
	return HandlerConfig{
		MaxSingleFileBytes: 25 << 20,
		MaxBulkFileCount:   100,
		MaxBulkTotalBytes:  250 << 20,
		MaxMissingHashes:   1000,
	}
}

// Handler exposes HTTP endpoints for run sync.
type Handler struct {
	service *Service
	cfg     HandlerConfig
}

// NewHandler constructs a sync handler.
func NewHandler(service *Service, cfg HandlerConfig) *Handler {
	defaults := DefaultHandlerConfig()
	if cfg.MaxSingleFileBytes <= 0 {
		cfg.MaxSingleFileBytes = defaults.MaxSingleFileBytes
	}
	if cfg.MaxBulkFileCount <= 0 {
		cfg.MaxBulkFileCount = defaults.MaxBulkFileCount
	}
	if cfg.MaxBulkTotalBytes <= 0 {
		cfg.MaxBulkTotalBytes = defaults.MaxBulkTotalBytes
	}
	if cfg.MaxMissingHashes <= 0 {
		cfg.MaxMissingHashes = defaults.MaxMissingHashes
	}

	return &Handler{service: service, cfg: cfg}
}

// HandleSync ingests one .refleks payload.
func (h *Handler) HandleSync(w http.ResponseWriter, r *http.Request) {
	fileName, raw, err := h.readSingleUpload(w, r)
	if err != nil {
		h.writeUploadError(w, err)
		return
	}
	_ = fileName

	result, err := h.service.SyncOne(r.Context(), raw)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// HandleBulkSync ingests multiple .refleks payloads in one request.
func (h *Handler) HandleBulkSync(w http.ResponseWriter, r *http.Request) {
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		writeError(w, http.StatusBadRequest, "bulk sync requires multipart/form-data with files[]")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxBulkTotalBytes+multipartOverheadBytes)
	if err := r.ParseMultipartForm(h.cfg.MaxBulkTotalBytes + multipartOverheadBytes); err != nil {
		h.writeUploadError(w, err)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		files = r.MultipartForm.File["file"]
	}
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "no files found: expected field files[]")
		return
	}
	if len(files) > h.cfg.MaxBulkFileCount {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("too many files: max is %d", h.cfg.MaxBulkFileCount))
		return
	}

	type itemResult struct {
		FileName       string `json:"file_name"`
		Hash           string `json:"hash,omitempty"`
		AlreadyPresent bool   `json:"already_present,omitempty"`
		Stored         bool   `json:"stored,omitempty"`
		Error          string `json:"error,omitempty"`
	}
	results := make([]itemResult, 0, len(files))

	var totalBytes int64
	for _, fh := range files {
		file, err := fh.Open()
		if err != nil {
			results = append(results, itemResult{FileName: fh.Filename, Error: "open upload: " + err.Error()})
			continue
		}

		raw, readErr := readBounded(file, h.cfg.MaxSingleFileBytes)
		_ = file.Close()
		if readErr != nil {
			results = append(results, itemResult{FileName: fh.Filename, Error: readErr.Error()})
			continue
		}

		totalBytes += int64(len(raw))
		if totalBytes > h.cfg.MaxBulkTotalBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "bulk payload exceeds maximum total bytes")
			return
		}

		syncResult, syncErr := h.service.SyncOne(r.Context(), raw)
		if syncErr != nil {
			results = append(results, itemResult{FileName: fh.Filename, Error: syncErr.Error()})
			continue
		}

		results = append(results, itemResult{
			FileName:       syncResult.FileName,
			Hash:           syncResult.Hash,
			AlreadyPresent: syncResult.AlreadyPresent,
			Stored:         syncResult.Stored,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results": results,
		"count":   len(results),
	})
}

// HandleMissingHashes returns which client-provided hashes are not yet synced.
func (h *Handler) HandleMissingHashes(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var req struct {
		Hashes []string `json:"hashes"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	if len(req.Hashes) > h.cfg.MaxMissingHashes {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("too many hashes: max is %d", h.cfg.MaxMissingHashes))
		return
	}

	missing, err := h.service.MissingHashes(r.Context(), req.Hashes)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"missing_hashes": missing,
	})
}

// HandleGetRun returns metadata for a single run by its hash.
func (h *Handler) HandleGetRun(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimSpace(r.PathValue("hash"))
	if hash == "" {
		hash = strings.TrimSpace(r.URL.Query().Get("hash"))
	}

	run, err := h.service.GetRun(r.Context(), hash)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, run)
}

// HandleDownloadRaw streams one raw .refleks file from object storage.
func (h *Handler) HandleDownloadRaw(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimSpace(r.PathValue("hash"))
	if hash == "" {
		hash = strings.TrimSpace(r.URL.Query().Get("hash"))
	}

	download, err := h.service.DownloadRaw(r.Context(), hash)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	defer download.Body.Close()

	contentType := download.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+download.FileName+"\"")
	if download.SizeBytes >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(download.SizeBytes, 10))
	}
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, download.Body); err != nil {
		return
	}
}

// HandleDownloadRawURL returns a URL for downloading one raw .refleks file.
func (h *Handler) HandleDownloadRawURL(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimSpace(r.PathValue("hash"))
	if hash == "" {
		hash = strings.TrimSpace(r.URL.Query().Get("hash"))
	}

	link, err := h.service.DownloadRawURL(r.Context(), hash)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	resp := map[string]any{
		"hash":      link.Hash,
		"file_name": link.FileName,
		"url":       link.URL,
		"access":    link.Access,
	}
	if link.ExpiresAt != nil {
		resp["expires_at"] = link.ExpiresAt.UTC().Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleListRuns returns filtered/sorted run metadata for frontend browsing.
func (h *Handler) HandleListRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit, err := optionalIntQuery(q.Get("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "limit must be an integer")
		return
	}
	offset, err := optionalIntQuery(q.Get("offset"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "offset must be an integer")
		return
	}
	minScore, err := optionalFloat64Query(q.Get("min_score"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "min_score must be a number")
		return
	}
	maxScore, err := optionalFloat64Query(q.Get("max_score"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "max_score must be a number")
		return
	}
	minAccuracy, err := optionalFloat64Query(q.Get("min_accuracy"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "min_accuracy must be a number")
		return
	}
	maxAccuracy, err := optionalFloat64Query(q.Get("max_accuracy"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "max_accuracy must be a number")
		return
	}
	fromPlayedAt, err := optionalInt64Query(q.Get("from_played_at"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "from_played_at must be an integer")
		return
	}
	toPlayedAt, err := optionalInt64Query(q.Get("to_played_at"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "to_played_at must be an integer")
		return
	}
	fromUploaded, err := optionalInt64Query(q.Get("from_uploaded"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "from_uploaded must be an integer")
		return
	}
	toUploaded, err := optionalInt64Query(q.Get("to_uploaded"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "to_uploaded must be an integer")
		return
	}
	hasMouseTrace, err := optionalBoolQuery(q.Get("has_mouse_trace"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "has_mouse_trace must be true or false")
		return
	}
	scenarioID, err := optionalInt64Query(q.Get("scenario_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "scenario_id must be an integer")
		return
	}

	var scenarioIDVal int64
	if scenarioID != nil {
		scenarioIDVal = *scenarioID
	}

	resp, err := h.service.ListRuns(r.Context(), RunListRequest{
		Limit:         limit,
		Offset:        offset,
		Sort:          RunsSort(strings.TrimSpace(q.Get("sort"))),
		ScenarioID:    scenarioIDVal,
		ScenarioName:  strings.TrimSpace(q.Get("scenario")),
		SteamID:       strings.TrimSpace(q.Get("steam_id")),
		SteamUsername: strings.TrimSpace(q.Get("steam_username")),
		Query:         strings.TrimSpace(q.Get("q")),
		HasMouseTrace: hasMouseTrace,
		MinScore:      minScore,
		MaxScore:      maxScore,
		MinAccuracy:   minAccuracy,
		MaxAccuracy:   maxAccuracy,
		FromPlayedAt:  fromPlayedAt,
		ToPlayedAt:    toPlayedAt,
		FromUploaded:  fromUploaded,
		ToUploaded:    toUploaded,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) readSingleUpload(w http.ResponseWriter, r *http.Request) (string, []byte, error) {
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxSingleFileBytes+multipartOverheadBytes)
		if err := r.ParseMultipartForm(h.cfg.MaxSingleFileBytes + multipartOverheadBytes); err != nil {
			return "", nil, err
		}

		file, fh, err := r.FormFile("file")
		if err != nil {
			return "", nil, fmt.Errorf("missing form field file")
		}
		defer file.Close()

		raw, err := readBounded(file, h.cfg.MaxSingleFileBytes)
		if err != nil {
			return "", nil, err
		}
		return fh.Filename, raw, nil
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxSingleFileBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return "", nil, err
	}
	fileName := strings.TrimSpace(r.URL.Query().Get("file_name"))
	return fileName, raw, nil
}

func (h *Handler) writeUploadError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "payload too large")
	case strings.Contains(strings.ToLower(err.Error()), "too large"):
		writeError(w, http.StatusRequestEntityTooLarge, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidRunFile):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrInvalidHash):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrObjectNotFound):
		writeError(w, http.StatusNotFound, "raw file not found")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func readBounded(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		return io.ReadAll(r)
	}
	limited := io.LimitReader(r, max+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(buf)) > max {
		return nil, fmt.Errorf("file too large")
	}
	return buf, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func optionalIntQuery(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

func optionalInt64Query(raw string) (*int64, error) {
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

func optionalFloat64Query(raw string) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func optionalBoolQuery(raw string) (*bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
