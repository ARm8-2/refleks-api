package runs

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	// ErrInvalidRunFile indicates the uploaded bytes are not a valid .refleks file.
	ErrInvalidRunFile = errors.New("invalid refleks file")
	// ErrInvalidHash indicates at least one provided hash has invalid format.
	ErrInvalidHash = errors.New("invalid hash")
	// ErrObjectNotFound indicates the raw file does not exist in object storage.
	ErrObjectNotFound = errors.New("object not found")
)

// Repository persists and queries sync metadata.
type Repository interface {
	ExistingHashes(ctx context.Context, hashes []string) (map[string]struct{}, error)
	InsertRun(ctx context.Context, meta RunMetadata) (bool, error)
	RunByHash(ctx context.Context, hash string) (StoredRun, error)
	RunDetail(ctx context.Context, hash string) (RunListItem, error)
	ListRuns(ctx context.Context, req RunListRequest) ([]RunListItem, error)
}

// StoredRun is the persisted lookup shape used for download operations.
type StoredRun struct {
	ObjectKey string
	FileName  string
}

// ObjectStore writes raw .refleks blobs.
type ObjectStore interface {
	Put(ctx context.Context, objectKey string, data []byte) error
	Get(ctx context.Context, objectKey string) (io.ReadCloser, int64, error)
	GetDownloadURL(ctx context.Context, objectKey, fileName string) (DownloadURL, error)
	Delete(ctx context.Context, objectKey string) error
}

// DownloadURL contains client-accessible object URL metadata.
type DownloadURL struct {
	URL       string
	Access    string
	ExpiresAt *time.Time
}
