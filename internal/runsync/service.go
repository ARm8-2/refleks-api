package runsync

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

	hash := sha256Hex(raw)
	existing, err := s.repo.ExistingHashes(ctx, []string{hash})
	if err != nil {
		return SyncResult{}, fmt.Errorf("query existing hash: %w", err)
	}
	if _, ok := existing[hash]; ok {
		return SyncResult{
			Hash:           hash,
			AlreadyPresent: true,
			Stored:         false,
			FileName:       parsed.FileName,
			EpochMilli:     parsed.EpochMilli,
			SizeBytes:      int64(len(raw)),
		}, nil
	}

	uploadedAt := s.now().UTC()
	objectKey := buildObjectKey(s.keyPrefix, parsed.EpochMilli, hash, uploadedAt)
	if err := s.store.Put(ctx, objectKey, raw); err != nil {
		return SyncResult{}, fmt.Errorf("store object: %w", err)
	}

	meta := RunMetadata{
		Hash:          hash,
		FileName:      parsed.FileName,
		ScenarioName:  strings.TrimSpace(parsed.ScenarioName),
		SteamID:       strings.TrimSpace(parsed.SteamID),
		SteamUsername: strings.TrimSpace(parsed.SteamUsername),
		EpochMilli:    parsed.EpochMilli,
		SizeBytes:     int64(len(raw)),
		ObjectKey:     objectKey,
		ChecksumHex:   checksumToHex(parsed.Checksum),
		UploadedAt:    uploadedAt,
		Compression:   parsed.Compression,
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
		return SyncResult{
			Hash:           hash,
			AlreadyPresent: true,
			Stored:         false,
			FileName:       parsed.FileName,
			EpochMilli:     parsed.EpochMilli,
			SizeBytes:      int64(len(raw)),
		}, nil
	}

	return SyncResult{
		Hash:           hash,
		AlreadyPresent: false,
		Stored:         true,
		FileName:       parsed.FileName,
		EpochMilli:     parsed.EpochMilli,
		SizeBytes:      int64(len(raw)),
	}, nil
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
		fileName = normalized + ".refleks"
	}

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
		fileName = normalized + ".refleks"
	}

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

func buildObjectKey(prefix string, epochMilli int64, hash string, fallback time.Time) string {
	name := hash + ".refleks"
	day := fallback.UTC()
	if epochMilli > 0 {
		day = time.UnixMilli(epochMilli).UTC()
	}
	datePath := day.Format("2006/01/02")
	if prefix == "" {
		return datePath + "/" + name
	}
	return prefix + "/" + datePath + "/" + name
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func checksumToHex(v uint64) string {
	var b [8]byte
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
	b[4] = byte(v >> 32)
	b[5] = byte(v >> 40)
	b[6] = byte(v >> 48)
	b[7] = byte(v >> 56)
	return hex.EncodeToString(b[:])
}
