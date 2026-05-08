package stats

import "context"

// Repository reads aggregate database totals.
type Repository interface {
	Counts(ctx context.Context) (Response, error)
}
