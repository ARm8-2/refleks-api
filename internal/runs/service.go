package runs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
)

var sha256HexRe = regexp.MustCompile(`^[a-f0-9]{64}$`)

const (
	defaultRunsListLimit = 50
	maxRunsListLimit     = 200
)

// Service orchestrates metadata persistence and blob storage for run sync.
type Service struct {
	repo      Repository
	store     ObjectStore
	keyPrefix string
	now       func() time.Time
}

// RawDownload contains a streamed object payload for a synced .refleks file.
type RawDownload struct {
	Hash        string
	FileName    string
	SizeBytes   int64
	Body        io.ReadCloser
	ContentType string
}

// RawDownloadLink contains an access URL for a synced .refleks file.
type RawDownloadLink struct {
	Hash      string
	FileName  string
	URL       string
	Access    string
	ExpiresAt *time.Time
}

// NewService constructs the run sync service.
func NewService(repo Repository, store ObjectStore, keyPrefix string) *Service {
	prefix := strings.TrimSpace(keyPrefix)
	prefix = strings.Trim(prefix, "/")

	return &Service{
		repo:      repo,
		store:     store,
		keyPrefix: prefix,
		now:       time.Now,
	}
}

// SyncOne verifies a .refleks file and stores it if not already present.
func (s *Service) SyncOne(ctx context.Context, raw []byte) (SyncResult, error) {
	if len(raw) == 0 {
		return SyncResult{}, fmt.Errorf("file is empty")
	}

	parsed, err := ParseRefleksFile(raw)
	if err != nil {
		return SyncResult{}, err
	}
	parsedFileName := ensureRunFileExtension(parsed.FileName)

	hash := sha256Hex(raw)
	sizeBytes := int64(len(raw))
	existing, err := s.repo.ExistingHashes(ctx, []string{hash})
	if err != nil {
		return SyncResult{}, fmt.Errorf("query existing hash: %w", err)
	}
	if _, ok := existing[hash]; ok {
		return duplicateSyncResult(hash, parsedFileName, parsed.PlayedAt, sizeBytes), nil
	}

	now := s.now().UTC()
	playedAt := time.UnixMilli(parsed.PlayedAt).UTC()
	objectKey := buildObjectKey(s.keyPrefix, playedAt, hash)
	if err := s.store.Put(ctx, objectKey, raw); err != nil {
		return SyncResult{}, fmt.Errorf("store object: %w", err)
	}

	meta := RunMetadata{
		Hash:          hash,
		FileName:      parsedFileName,
		ScenarioName:  strings.TrimSpace(parsed.ScenarioName),
		SteamID:       strings.TrimSpace(parsed.SteamID),
		SteamUsername: strings.TrimSpace(parsed.SteamUsername),
		PlayedAt:      playedAt,
		SizeBytes:     sizeBytes,
		ObjectKey:     objectKey,
		UploadedAt:    now,
		FormatVersion: parsed.FormatVersion,
		Score:         parsed.Score,
		Accuracy:      parsed.Accuracy,
		AvgTTKSeconds: parsed.AvgTTKSeconds,
		DurationSecs:  parsed.DurationSecs,
		SensCM360:     parsed.SensCM360,
		HasMouseTrace: parsed.HasMouseTrace,
		AvgMouseSpeed: parsed.AvgMouseSpeed,
		MouseVID:      strings.TrimSpace(parsed.MouseVID),
		MousePID:      strings.TrimSpace(parsed.MousePID),
	}

	inserted, err := s.repo.InsertRun(ctx, meta)
	if err != nil {
		_ = s.store.Delete(ctx, objectKey)
		return SyncResult{}, fmt.Errorf("persist metadata: %w", err)
	}

	if !inserted {
		_ = s.store.Delete(ctx, objectKey)
		return duplicateSyncResult(hash, parsedFileName, parsed.PlayedAt, sizeBytes), nil
	}

	return SyncResult{
		Hash:           hash,
		AlreadyPresent: false,
		Stored:         true,
		FileName:       parsedFileName,
		PlayedAt:       parsed.PlayedAt,
		SizeBytes:      sizeBytes,
	}, nil
}

func duplicateSyncResult(hash, fileName string, playedAt, sizeBytes int64) SyncResult {
	return SyncResult{
		Hash:           hash,
		AlreadyPresent: true,
		Stored:         false,
		FileName:       fileName,
		PlayedAt:       playedAt,
		SizeBytes:      sizeBytes,
	}
}

// MissingHashes returns hashes that do not exist.
func (s *Service) MissingHashes(ctx context.Context, hashes []string) ([]string, error) {
	normalized, err := normalizeHashes(hashes)
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 {
		return []string{}, nil
	}

	existing, err := s.repo.ExistingHashes(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("query existing hashes: %w", err)
	}

	missing := make([]string, 0, len(normalized))
	for _, hash := range normalized {
		if _, ok := existing[hash]; ok {
			continue
		}
		missing = append(missing, hash)
	}
	return missing, nil
}

// DownloadRaw returns a reader for a raw .refleks file.
func (s *Service) DownloadRaw(ctx context.Context, hash string) (RawDownload, error) {
	if s.store == nil {
		return RawDownload{}, fmt.Errorf("raw download not available")
	}
	normalized, err := normalizeSingleHash(hash)
	if err != nil {
		return RawDownload{}, err
	}

	run, err := s.repo.RunByHash(ctx, normalized)
	if err != nil {
		return RawDownload{}, err
	}
	body, size, err := s.store.Get(ctx, run.ObjectKey)
	if err != nil {
		return RawDownload{}, err
	}

	fileName := strings.TrimSpace(run.FileName)
	if fileName == "" {
		fileName = normalized + RunFileExtension
	}
	fileName = ensureRunFileExtension(fileName)

	return RawDownload{
		Hash:        normalized,
		FileName:    fileName,
		SizeBytes:   size,
		Body:        body,
		ContentType: "application/octet-stream",
	}, nil
}

// DownloadRawURL returns a URL for downloading a raw .refleks file.
func (s *Service) DownloadRawURL(ctx context.Context, hash string) (RawDownloadLink, error) {
	normalized, err := normalizeSingleHash(hash)
	if err != nil {
		return RawDownloadLink{}, err
	}

	run, err := s.repo.RunByHash(ctx, normalized)
	if err != nil {
		return RawDownloadLink{}, err
	}

	fileName := strings.TrimSpace(run.FileName)
	if fileName == "" {
		fileName = normalized + RunFileExtension
	}
	fileName = ensureRunFileExtension(fileName)

	downloadURL, err := s.store.GetDownloadURL(ctx, run.ObjectKey, fileName)
	if err != nil {
		return RawDownloadLink{}, err
	}

	return RawDownloadLink{
		Hash:      normalized,
		FileName:  fileName,
		URL:       downloadURL.URL,
		Access:    downloadURL.Access,
		ExpiresAt: downloadURL.ExpiresAt,
	}, nil
}

// GetRun returns metadata for a single run by its SHA-256 hash.
func (s *Service) GetRun(ctx context.Context, hash string) (RunListItem, error) {
	normalized, err := normalizeSingleHash(hash)
	if err != nil {
		return RunListItem{}, err
	}
	return s.repo.RunDetail(ctx, normalized)
}

// ListRuns returns filtered, sorted, paginated run metadata for frontend browsing.
func (s *Service) ListRuns(ctx context.Context, req RunListRequest) (RunListResponse, error) {
	query := normalizeRunListRequest(req)
	query.Limit++

	runs, err := s.repo.ListRuns(ctx, query)
	if err != nil {
		return RunListResponse{}, fmt.Errorf("list runs: %w", err)
	}

	effectiveLimit := query.Limit - 1
	hasMore := len(runs) > effectiveLimit
	if hasMore {
		runs = runs[:effectiveLimit]
	}

	resp := RunListResponse{
		Runs:    runs,
		Limit:   effectiveLimit,
		Offset:  query.Offset,
		Count:   len(runs),
		HasMore: hasMore,
	}
	if hasMore {
		nextOffset := query.Offset + len(runs)
		resp.NextOffset = &nextOffset
	}

	return resp, nil
}

func normalizeHashes(hashes []string) ([]string, error) {
	if len(hashes) == 0 {
		return []string{}, nil
	}

	seen := make(map[string]struct{}, len(hashes))
	normalized := make([]string, 0, len(hashes))
	for _, raw := range hashes {
		hash := strings.ToLower(strings.TrimSpace(raw))
		if hash == "" {
			continue
		}
		if !sha256HexRe.MatchString(hash) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidHash, raw)
		}
		if _, ok := seen[hash]; ok {
			continue
		}
		seen[hash] = struct{}{}
		normalized = append(normalized, hash)
	}

	// Keep responses deterministic for clients regardless of input ordering.
	sort.Strings(normalized)
	return normalized, nil
}

func normalizeSingleHash(hash string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(hash))
	if !sha256HexRe.MatchString(normalized) {
		return "", fmt.Errorf("%w: %s", ErrInvalidHash, hash)
	}
	return normalized, nil
}

func normalizeRunListRequest(req RunListRequest) RunListRequest {
	out := req
	out.ScenarioName = strings.TrimSpace(out.ScenarioName)
	out.SteamID = strings.TrimSpace(out.SteamID)
	out.SteamUsername = strings.TrimSpace(out.SteamUsername)
	out.Query = strings.TrimSpace(out.Query)

	if out.Limit <= 0 {
		out.Limit = defaultRunsListLimit
	}
	if out.Limit > maxRunsListLimit {
		out.Limit = maxRunsListLimit
	}
	if out.Offset < 0 {
		out.Offset = 0
	}

	switch out.Sort {
	case RunsSortUploadedAtAsc, RunsSortUploadedAtDesc,
		RunsSortPlayedAtAsc, RunsSortPlayedAtDesc,
		RunsSortScoreAsc, RunsSortScoreDesc,
		RunsSortAccuracyAsc, RunsSortAccuracyDesc,
		RunsSortAvgTTKAsc, RunsSortAvgTTKDesc:
	default:
		out.Sort = RunsSortUploadedAtDesc
	}

	if out.MinScore != nil && out.MaxScore != nil && *out.MinScore > *out.MaxScore {
		out.MinScore, out.MaxScore = out.MaxScore, out.MinScore
	}
	if out.MinAccuracy != nil && out.MaxAccuracy != nil && *out.MinAccuracy > *out.MaxAccuracy {
		out.MinAccuracy, out.MaxAccuracy = out.MaxAccuracy, out.MinAccuracy
	}
	if out.FromPlayedAt != nil && out.ToPlayedAt != nil && *out.FromPlayedAt > *out.ToPlayedAt {
		out.FromPlayedAt, out.ToPlayedAt = out.ToPlayedAt, out.FromPlayedAt
	}
	if out.FromUploaded != nil && out.ToUploaded != nil && *out.FromUploaded > *out.ToUploaded {
		out.FromUploaded, out.ToUploaded = out.ToUploaded, out.FromUploaded
	}

	return out
}

func buildObjectKey(prefix string, playedAt time.Time, hash string) string {
	name := hash + RunFileExtension
	datePath := playedAt.Format("2006/01/02")
	if prefix == "" {
		return datePath + "/" + name
	}
	return prefix + "/" + datePath + "/" + name
}

func ensureRunFileExtension(fileName string) string {
	trimmed := strings.TrimSpace(fileName)
	if trimmed == "" {
		return trimmed
	}
	if strings.HasSuffix(strings.ToLower(trimmed), RunFileExtension) {
		return trimmed
	}
	return trimmed + RunFileExtension
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
