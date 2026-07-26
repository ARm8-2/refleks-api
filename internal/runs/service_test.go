package runs

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/zeebo/xxh3"
	"google.golang.org/protobuf/encoding/protowire"
)

type memRepo struct {
	runs map[string]RunMetadata
}

func newMemRepo() *memRepo {
	return &memRepo{runs: make(map[string]RunMetadata)}
}

func (m *memRepo) ExistingHashes(_ context.Context, hashes []string) (map[string]struct{}, error) {
	found := make(map[string]struct{})
	for _, hash := range hashes {
		if _, ok := m.runs[hash]; ok {
			found[hash] = struct{}{}
		}
	}
	return found, nil
}

func (m *memRepo) InsertRun(_ context.Context, meta RunMetadata) (bool, error) {
	if _, exists := m.runs[meta.Hash]; exists {
		return false, nil
	}
	m.runs[meta.Hash] = meta
	return true, nil
}

func (m *memRepo) RunDetail(_ context.Context, hash string) (RunListItem, error) {
	meta, ok := m.runs[hash]
	if !ok {
		return RunListItem{}, ErrObjectNotFound
	}
	return RunListItem{
		Hash:          meta.Hash,
		FileName:      meta.FileName,
		ScenarioName:  meta.ScenarioName,
		SteamID:       meta.SteamID,
		SteamUsername: meta.SteamUsername,
		PlayedAt:      meta.PlayedAt.UnixMilli(),
		UploadedAt:    meta.UploadedAt.UnixMilli(),
		SizeBytes:     meta.SizeBytes,
		Score:         meta.Score,
		Accuracy:      meta.Accuracy,
		AvgTTKSeconds: meta.AvgTTKSeconds,
		DurationSecs:  meta.DurationSecs,
		SensCM360:     meta.SensCM360,
		HasMouseTrace: meta.HasMouseTrace,
		AvgMouseSpeed: meta.AvgMouseSpeed,
		MouseVID:      meta.MouseVID,
		MousePID:      meta.MousePID,
	}, nil
}

func (m *memRepo) RunByHash(_ context.Context, hash string) (StoredRun, error) {
	meta, ok := m.runs[hash]
	if !ok {
		return StoredRun{}, ErrObjectNotFound
	}
	return StoredRun{ObjectKey: meta.ObjectKey, FileName: meta.FileName}, nil
}

func (m *memRepo) ListRuns(_ context.Context, req RunListRequest) ([]RunListItem, error) {
	items := make([]RunListItem, 0, len(m.runs))
	for _, meta := range m.runs {
		item := RunListItem{
			Hash:          meta.Hash,
			FileName:      meta.FileName,
			ScenarioName:  meta.ScenarioName,
			SteamID:       meta.SteamID,
			SteamUsername: meta.SteamUsername,
			PlayedAt:      meta.PlayedAt.UnixMilli(),
			UploadedAt:    meta.UploadedAt.UnixMilli(),
			SizeBytes:     meta.SizeBytes,
			Score:         meta.Score,
			Accuracy:      meta.Accuracy,
			AvgTTKSeconds: meta.AvgTTKSeconds,
			DurationSecs:  meta.DurationSecs,
			SensCM360:     meta.SensCM360,
			HasMouseTrace: meta.HasMouseTrace,
			AvgMouseSpeed: meta.AvgMouseSpeed,
			MouseVID:      meta.MouseVID,
			MousePID:      meta.MousePID,
		}

		if req.ScenarioName != "" && !containsFold(item.ScenarioName, req.ScenarioName) {
			continue
		}
		if req.SteamID != "" && !containsFold(item.SteamID, req.SteamID) {
			continue
		}
		if req.SteamUsername != "" && !containsFold(item.SteamUsername, req.SteamUsername) {
			continue
		}
		if req.Query != "" &&
			!containsFold(item.FileName, req.Query) &&
			!containsFold(item.ScenarioName, req.Query) &&
			!containsFold(item.SteamUsername, req.Query) {
			continue
		}
		if req.HasMouseTrace != nil && item.HasMouseTrace != *req.HasMouseTrace {
			continue
		}
		if req.MinScore != nil {
			if item.Score == nil || *item.Score < *req.MinScore {
				continue
			}
		}
		if req.MaxScore != nil {
			if item.Score == nil || *item.Score > *req.MaxScore {
				continue
			}
		}
		if req.FromPlayedAt != nil && item.PlayedAt < *req.FromPlayedAt {
			continue
		}
		if req.ToPlayedAt != nil && item.PlayedAt > *req.ToPlayedAt {
			continue
		}
		if req.FromUploaded != nil && item.UploadedAt < *req.FromUploaded {
			continue
		}
		if req.ToUploaded != nil && item.UploadedAt > *req.ToUploaded {
			continue
		}

		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		a := items[i]
		b := items[j]
		switch req.Sort {
		case RunsSortUploadedAtAsc:
			if a.UploadedAt == b.UploadedAt {
				return a.Hash < b.Hash
			}
			return a.UploadedAt < b.UploadedAt
		case RunsSortPlayedAtDesc:
			if a.PlayedAt == b.PlayedAt {
				return a.Hash < b.Hash
			}
			return a.PlayedAt > b.PlayedAt
		case RunsSortPlayedAtAsc:
			if a.PlayedAt == b.PlayedAt {
				return a.Hash < b.Hash
			}
			return a.PlayedAt < b.PlayedAt
		case RunsSortScoreDesc:
			av := scoreOrNegInf(a.Score)
			bv := scoreOrNegInf(b.Score)
			if av == bv {
				return a.Hash < b.Hash
			}
			return av > bv
		case RunsSortScoreAsc:
			av := scoreOrNegInf(a.Score)
			bv := scoreOrNegInf(b.Score)
			if av == bv {
				return a.Hash < b.Hash
			}
			return av < bv
		default:
			if a.UploadedAt == b.UploadedAt {
				return a.Hash < b.Hash
			}
			return a.UploadedAt > b.UploadedAt
		}
	})

	if req.Offset >= len(items) {
		return []RunListItem{}, nil
	}
	items = items[req.Offset:]
	if req.Limit > 0 && len(items) > req.Limit {
		items = items[:req.Limit]
	}

	return items, nil
}

type memStore struct {
	objects map[string][]byte
	puts    int
}

func newMemStore() *memStore {
	return &memStore{objects: make(map[string][]byte)}
}

func (m *memStore) Put(_ context.Context, objectKey string, data []byte) error {
	m.puts++
	m.objects[objectKey] = append([]byte(nil), data...)
	return nil
}

func (m *memStore) Get(_ context.Context, objectKey string) (io.ReadCloser, int64, error) {
	raw, ok := m.objects[objectKey]
	if !ok {
		return nil, 0, ErrObjectNotFound
	}
	copyRaw := append([]byte(nil), raw...)
	return io.NopCloser(bytes.NewReader(copyRaw)), int64(len(copyRaw)), nil
}

func (m *memStore) GetDownloadURL(_ context.Context, objectKey, _ string) (DownloadURL, error) {
	if _, ok := m.objects[objectKey]; !ok {
		return DownloadURL{}, ErrObjectNotFound
	}
	return DownloadURL{
		URL:    "https://example.test/" + objectKey,
		Access: "public",
	}, nil
}

func (m *memStore) Delete(_ context.Context, objectKey string) error {
	delete(m.objects, objectKey)
	return nil
}

func TestServiceSyncOne_DeduplicatesByHash(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	store := newMemStore()
	svc := NewService(repo, store, "runs")
	svc.now = func() time.Time {
		return time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	}

	raw := buildTestRefleksFile(t, "sample.refleks", 1742640000000)
	first, err := svc.SyncOne(context.Background(), raw)
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	if first.AlreadyPresent {
		t.Fatalf("expected first sync not to be already present")
	}
	if !first.Stored {
		t.Fatalf("expected first sync to store object")
	}
	if first.Hash == "" {
		t.Fatalf("expected hash in response")
	}

	second, err := svc.SyncOne(context.Background(), raw)
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}
	if !second.AlreadyPresent {
		t.Fatalf("expected second sync to be already present")
	}
	if second.Stored {
		t.Fatalf("expected second sync not to write metadata")
	}
	if store.puts != 1 {
		t.Fatalf("expected exactly one object upload, got %d", store.puts)
	}
}

func TestServiceMissingHashes_ReturnsOnlyNotPersisted(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	store := newMemStore()
	svc := NewService(repo, store, "runs")

	raw := buildTestRefleksFile(t, "sample.refleks", 1742640000000)
	inserted, err := svc.SyncOne(context.Background(), raw)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	missingHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	missing, err := svc.MissingHashes(context.Background(), []string{inserted.Hash, missingHash, inserted.Hash})
	if err != nil {
		t.Fatalf("missing hashes failed: %v", err)
	}
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing hash, got %d", len(missing))
	}
	if missing[0] != missingHash {
		t.Fatalf("expected %s, got %s", missingHash, missing[0])
	}
}

func TestServiceMissingHashes_InvalidHash(t *testing.T) {
	t.Parallel()

	svc := NewService(newMemRepo(), newMemStore(), "runs")
	_, err := svc.MissingHashes(context.Background(), []string{"not-a-hash"})
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("expected ErrInvalidHash, got %v", err)
	}
}

func TestServiceDownloadRaw_Success(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	store := newMemStore()
	svc := NewService(repo, store, "runs")

	raw := buildTestRefleksFile(t, "download.refleks", 1742640000000)
	synced, err := svc.SyncOne(context.Background(), raw)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	download, err := svc.DownloadRaw(context.Background(), synced.Hash)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	defer download.Body.Close()

	body, err := io.ReadAll(download.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(body, raw) {
		t.Fatalf("downloaded payload mismatch")
	}
	if download.FileName != synced.FileName {
		t.Fatalf("unexpected filename: %s", download.FileName)
	}
}

func TestServiceDownloadRaw_NotFound(t *testing.T) {
	t.Parallel()

	svc := NewService(newMemRepo(), newMemStore(), "runs")
	_, err := svc.DownloadRaw(context.Background(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("expected ErrObjectNotFound, got %v", err)
	}
}

func TestServiceDownloadRawURL_Success(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	store := newMemStore()
	svc := NewService(repo, store, "runs")

	raw := buildTestRefleksFile(t, "download-url.refleks", 1742640000000)
	synced, err := svc.SyncOne(context.Background(), raw)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	link, err := svc.DownloadRawURL(context.Background(), synced.Hash)
	if err != nil {
		t.Fatalf("download url failed: %v", err)
	}
	if link.URL == "" {
		t.Fatalf("expected url in response")
	}
	if link.Access != "public" {
		t.Fatalf("unexpected access mode: %s", link.Access)
	}
	if link.FileName != synced.FileName {
		t.Fatalf("unexpected download filename: %s", link.FileName)
	}
}

func TestServiceListRuns_FilterSortPagination(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	repo.runs["h1"] = RunMetadata{
		Hash:          "h1",
		FileName:      "run-1.refleks",
		ScenarioName:  "VT Pat",
		SteamUsername: "alice",
		PlayedAt:      time.UnixMilli(2000),
		UploadedAt:    time.UnixMilli(1774519200000),
		Score:         float64PtrTest(88),
	}
	repo.runs["h2"] = RunMetadata{
		Hash:          "h2",
		FileName:      "run-2.refleks",
		ScenarioName:  "VT Pat",
		SteamUsername: "alice",
		PlayedAt:      time.UnixMilli(3000),
		UploadedAt:    time.UnixMilli(1774522800000),
		Score:         float64PtrTest(99),
	}
	repo.runs["h3"] = RunMetadata{
		Hash:          "h3",
		FileName:      "run-3.refleks",
		ScenarioName:  "Other",
		SteamUsername: "bob",
		PlayedAt:      time.UnixMilli(1000),
		UploadedAt:    time.UnixMilli(1774515600000),
		Score:         float64PtrTest(70),
	}

	svc := NewService(repo, newMemStore(), "runs")
	resp, err := svc.ListRuns(context.Background(), RunListRequest{
		Limit:        1,
		Sort:         RunsSortScoreDesc,
		ScenarioName: "vt",
		Query:        "alice",
		MinScore:     float64PtrTest(80),
	})
	if err != nil {
		t.Fatalf("list runs failed: %v", err)
	}
	if resp.Count != 1 {
		t.Fatalf("expected count 1, got %d", resp.Count)
	}
	if !resp.HasMore {
		t.Fatalf("expected has_more=true")
	}
	if len(resp.Runs) != 1 || resp.Runs[0].Hash != "h2" {
		t.Fatalf("unexpected top run: %#v", resp.Runs)
	}
}

func TestServiceSyncOne_AppendsMissingFileExtension(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	store := newMemStore()
	svc := NewService(repo, store, "runs")

	raw := buildTestRefleksFile(t, "no-extension", 1742640000000)
	result, err := svc.SyncOne(context.Background(), raw)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if result.FileName != "no-extension"+RunFileExtension {
		t.Fatalf("expected filename with extension, got %q", result.FileName)
	}

	stored, ok := repo.runs[result.Hash]
	if !ok {
		t.Fatalf("expected run in repo")
	}
	if stored.FileName != "no-extension"+RunFileExtension {
		t.Fatalf("expected stored filename with extension, got %q", stored.FileName)
	}
}

type lateConflictRepo struct{}

func (lateConflictRepo) ExistingHashes(context.Context, []string) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}

func (lateConflictRepo) InsertRun(context.Context, RunMetadata) (bool, error) {
	return false, nil
}

func (lateConflictRepo) RunByHash(context.Context, string) (StoredRun, error) {
	return StoredRun{}, ErrObjectNotFound
}

func (lateConflictRepo) RunDetail(context.Context, string) (RunListItem, error) {
	return RunListItem{}, ErrObjectNotFound
}

func (lateConflictRepo) ListRuns(context.Context, RunListRequest) ([]RunListItem, error) {
	return nil, nil
}

func TestServiceSyncOne_DeletesUploadedObjectWhenInsertConflicts(t *testing.T) {
	t.Parallel()

	store := newMemStore()
	svc := NewService(lateConflictRepo{}, store, "runs")

	raw := buildTestRefleksFile(t, "sample.refleks", 1742640000000)
	result, err := svc.SyncOne(context.Background(), raw)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	if !result.AlreadyPresent || result.Stored {
		t.Fatalf("expected duplicate result after insert conflict, got %#v", result)
	}
	if len(store.objects) != 0 {
		t.Fatalf("expected uploaded object to be cleaned up, got %d objects", len(store.objects))
	}
}

type testStatEntry struct {
	Key    string
	Type   uint8
	String string
	Int    int64
	Float  float64
	Bool   bool
}

func TestServiceGetRun_Success(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	store := newMemStore()
	svc := NewService(repo, store, "runs")

	raw := buildTestRefleksFile(t, "detail.refleks", 1742640000000)
	synced, err := svc.SyncOne(context.Background(), raw)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	run, err := svc.GetRun(context.Background(), synced.Hash)
	if err != nil {
		t.Fatalf("get run failed: %v", err)
	}
	if run.Hash != synced.Hash {
		t.Fatalf("expected hash %s, got %s", synced.Hash, run.Hash)
	}
	if run.FileName != synced.FileName {
		t.Fatalf("expected file name %s, got %s", synced.FileName, run.FileName)
	}
}

func TestServiceGetRun_NotFound(t *testing.T) {
	t.Parallel()

	svc := NewService(newMemRepo(), newMemStore(), "runs")
	_, err := svc.GetRun(context.Background(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("expected ErrObjectNotFound, got %v", err)
	}
}

func TestServiceGetRun_InvalidHash(t *testing.T) {
	t.Parallel()

	svc := NewService(newMemRepo(), newMemStore(), "runs")
	_, err := svc.GetRun(context.Background(), "not-a-hash")
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("expected ErrInvalidHash, got %v", err)
	}
}

func float64PtrTest(v float64) *float64 {
	copy := v
	return &copy
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func scoreOrNegInf(v *float64) float64 {
	if v == nil {
		return math.Inf(-1)
	}
	return *v
}

func buildTestRefleksFile(t *testing.T, fileName string, epochMilli int64) []byte {
	return buildTestRefleksFileWithStats(t, fileName, epochMilli, nil)
}

// testStatToField maps human-readable stat keys to protowire field numbers.
var testStatToField = map[string]struct {
	field    protowire.Number
	isVarint bool
}{
	"Scenario":     {field: 19},
	"Score":        {field: 1},
	"Accuracy":     {field: 44},
	"Real Avg TTK": {field: 45},
	"Avg TTK":      {field: 6},
	"Duration":     {field: 47},
	"cm/360":       {field: 46},
	"Kills":        {field: 2, isVarint: true},
	"Deaths":       {field: 3, isVarint: true},
	"Damage Done":  {field: 7},
	"Hit Count":    {field: 10, isVarint: true},
}

func buildTestRefleksFileWithStats(t *testing.T, fileName string, epochMilli int64, stats []testStatEntry) []byte {
	t.Helper()

	// Build stats section payload (protowire RunStatsData, field 1 = summary message)
	var statsPayload []byte
	if len(stats) > 0 {
		// Build summary message (RunStatsSummary)
		var summaryBuf []byte
		for _, stat := range stats {
			mapping, ok := testStatToField[stat.Key]
			if !ok {
				continue
			}
			switch stat.Type {
			case 1: // statTypeString (string)
				summaryBuf = protowire.AppendTag(summaryBuf, mapping.field, protowire.BytesType)
				summaryBuf = protowire.AppendString(summaryBuf, stat.String)
			case 2, 4: // statTypeInt or statTypeBool
				summaryBuf = protowire.AppendTag(summaryBuf, mapping.field, protowire.VarintType)
				summaryBuf = protowire.AppendVarint(summaryBuf, uint64(stat.Int))
			case 3: // statTypeFloat
				summaryBuf = protowire.AppendTag(summaryBuf, mapping.field, protowire.Fixed64Type)
				summaryBuf = protowire.AppendFixed64(summaryBuf, math.Float64bits(stat.Float))
			}
		}
		// Wrap summary in RunStatsData field 1
		statsPayload = protowire.AppendTag(nil, 1, protowire.BytesType)
		statsPayload = protowire.AppendBytes(statsPayload, summaryBuf)
	}

	payload := new(bytes.Buffer)
	writeString := func(s string) {
		t.Helper()
		if err := binary.Write(payload, binary.LittleEndian, uint32(len(s))); err != nil {
			t.Fatalf("write string size: %v", err)
		}
		if _, err := payload.WriteString(s); err != nil {
			t.Fatalf("write string body: %v", err)
		}
	}

	// Section 1: file name
	writeString(fileName)

	// Section 2: stats
	if err := binary.Write(payload, binary.LittleEndian, uint32(len(statsPayload))); err != nil {
		t.Fatalf("write stats size: %v", err)
	}
	if len(statsPayload) > 0 {
		if _, err := payload.Write(statsPayload); err != nil {
			t.Fatalf("write stats body: %v", err)
		}
	}

	// Section 3: performances (empty)
	if err := binary.Write(payload, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatalf("write performances size: %v", err)
	}

	// Section 4: mouse trace (empty)
	if err := binary.Write(payload, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatalf("write trace size: %v", err)
	}

	// Section 5: environment (all empty)
	if err := binary.Write(payload, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatalf("write env size: %v", err)
	}

	encodedPayload := payload.Bytes()
	checksum := xxh3.Hash(encodedPayload)

	out := new(bytes.Buffer)
	if _, err := out.Write([]byte(runMagic)); err != nil {
		t.Fatalf("write magic: %v", err)
	}
	if err := binary.Write(out, binary.LittleEndian, uint8(runVersion)); err != nil {
		t.Fatalf("write version: %v", err)
	}
	if err := binary.Write(out, binary.LittleEndian, uint8(runCompressionNone)); err != nil {
		t.Fatalf("write compression: %v", err)
	}
	if err := binary.Write(out, binary.LittleEndian, epochMilli); err != nil {
		t.Fatalf("write epoch: %v", err)
	}
	if _, err := out.Write(encodedPayload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	if err := binary.Write(out, binary.LittleEndian, checksum); err != nil {
		t.Fatalf("write checksum: %v", err)
	}

	return out.Bytes()
}
