package scenarios

import (
	"context"
	"errors"
)

var (
	// ErrNotFound indicates the requested scenario does not exist.
	ErrNotFound = errors.New("scenario not found")
)

// Repository reads scenario metadata from storage.
type Repository interface {
	ListScenarios(ctx context.Context, req ListRequest) ([]ScenarioListItem, error)
	ScenarioByID(ctx context.Context, id int64) (ScenarioDetail, error)
}
