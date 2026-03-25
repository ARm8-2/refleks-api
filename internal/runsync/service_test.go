package runsync

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/zeebo/xxh3"
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

func (m *memRepo) RunByHash(_ context.Context, hash string) (StoredRun, error) {
	meta, ok := m.runs[hash]
	if !ok {
		return StoredRun{}, ErrObjectNotFound
	}
	return StoredRun{ObjectKey: meta.ObjectKey, FileName: meta.FileName}, nil
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

func buildTestRefleksFile(t *testing.T, fileName string, epochMilli int64) []byte {
	t.Helper()

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

	writeString(fileName)
	if err := binary.Write(payload, binary.LittleEndian, uint32(0)); err != nil { // stats
		t.Fatalf("write stats len: %v", err)
	}
	if err := binary.Write(payload, binary.LittleEndian, uint32(0)); err != nil { // events
		t.Fatalf("write events rows: %v", err)
	}
	if err := binary.Write(payload, binary.LittleEndian, uint32(0)); err != nil { // mouse trace
		t.Fatalf("write trace len: %v", err)
	}

	for i := 0; i < 8; i++ { // first 8 env strings
		writeString("")
	}
	if err := binary.Write(payload, binary.LittleEndian, int32(0)); err != nil { // cpu cores
		t.Fatalf("write cpu cores: %v", err)
	}
	writeString("")                                                              // gpu name
	if err := binary.Write(payload, binary.LittleEndian, int32(0)); err != nil { // ram
		t.Fatalf("write ram: %v", err)
	}
	if err := binary.Write(payload, binary.LittleEndian, float64(0)); err != nil { // hz
		t.Fatalf("write display hz: %v", err)
	}
	if err := binary.Write(payload, binary.LittleEndian, int32(0)); err != nil { // w
		t.Fatalf("write width: %v", err)
	}
	if err := binary.Write(payload, binary.LittleEndian, int32(0)); err != nil { // h
		t.Fatalf("write height: %v", err)
	}
	if err := binary.Write(payload, binary.LittleEndian, uint8(0)); err != nil { // windowed
		t.Fatalf("write windowed: %v", err)
	}
	for i := 0; i < 5; i++ { // mouse strings
		writeString("")
	}
	if err := binary.Write(payload, binary.LittleEndian, int32(0)); err != nil { // trace points
		t.Fatalf("write trace points: %v", err)
	}
	if err := binary.Write(payload, binary.LittleEndian, float64(0)); err != nil { // duration
		t.Fatalf("write trace duration: %v", err)
	}
	if err := binary.Write(payload, binary.LittleEndian, int32(0)); err != nil { // sample rate
		t.Fatalf("write sample rate: %v", err)
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
