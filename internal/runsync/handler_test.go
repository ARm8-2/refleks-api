package runsync

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerHandleSync_OctetStream(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(newMemRepo(), newMemStore(), "runs"), DefaultHandlerConfig())
	raw := buildTestRefleksFile(t, "single.refleks", 1742640000000)

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/sync", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/octet-stream")
	rec := httptest.NewRecorder()

	h.HandleSync(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body SyncResult
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Hash == "" {
		t.Fatalf("expected hash in response")
	}
	if !body.Stored {
		t.Fatalf("expected stored=true")
	}
}

func TestHandlerHandleMissingHashes(t *testing.T) {
	t.Parallel()

	svc := NewService(newMemRepo(), newMemStore(), "runs")
	h := NewHandler(svc, DefaultHandlerConfig())

	raw := buildTestRefleksFile(t, "existing.refleks", 1742640000000)
	synced, err := svc.SyncOne(context.Background(), raw)
	if err != nil {
		t.Fatalf("seed sync failed: %v", err)
	}

	missingHash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	payload := map[string]any{
		"hashes": []string{synced.Hash, missingHash},
	}
	encoded, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/v1/runs/sync/missing", bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.HandleMissingHashes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		MissingHashes []string `json:"missing_hashes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.MissingHashes) != 1 || resp.MissingHashes[0] != missingHash {
		t.Fatalf("unexpected missing hashes: %#v", resp.MissingHashes)
	}
}

func TestHandlerHandleDownloadRaw(t *testing.T) {
	t.Parallel()

	svc := NewService(newMemRepo(), newMemStore(), "runs")
	h := NewHandler(svc, DefaultHandlerConfig())

	raw := buildTestRefleksFile(t, "download.refleks", 1742640000000)
	synced, err := svc.SyncOne(context.Background(), raw)
	if err != nil {
		t.Fatalf("seed sync failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/raw/"+synced.Hash, nil)
	req.SetPathValue("hash", synced.Hash)
	rec := httptest.NewRecorder()

	h.HandleDownloadRaw(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("unexpected content type: %s", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got == "" {
		t.Fatalf("expected content disposition header")
	}

	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !bytes.Equal(body, raw) {
		t.Fatalf("download payload mismatch")
	}
}

func TestHandlerHandleDownloadRawURL(t *testing.T) {
	t.Parallel()

	svc := NewService(newMemRepo(), newMemStore(), "runs")
	h := NewHandler(svc, DefaultHandlerConfig())

	raw := buildTestRefleksFile(t, "download-url.refleks", 1742640000000)
	synced, err := svc.SyncOne(context.Background(), raw)
	if err != nil {
		t.Fatalf("seed sync failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/runs/raw/"+synced.Hash+"/url", nil)
	req.SetPathValue("hash", synced.Hash)
	rec := httptest.NewRecorder()

	h.HandleDownloadRawURL(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body struct {
		Hash   string `json:"hash"`
		URL    string `json:"url"`
		Access string `json:"access"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Hash != synced.Hash {
		t.Fatalf("unexpected hash: %s", body.Hash)
	}
	if body.URL == "" {
		t.Fatalf("expected url in response")
	}
	if body.Access != "public" {
		t.Fatalf("unexpected access type: %s", body.Access)
	}
}
