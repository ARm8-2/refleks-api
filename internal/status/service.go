package status

import "time"

// Service holds runtime metadata for status checks.
type Service struct {
	appName   string
	env       string
	version   string
	startedAt time.Time
}

// Response is the API status payload.
type Response struct {
	Status        string `json:"status"`
	Service       string `json:"service"`
	Environment   string `json:"environment"`
	Version       string `json:"version"`
	Timestamp     string `json:"timestamp"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// NewService constructs a status service.
func NewService(appName, env, version string, startedAt time.Time) *Service {
	if startedAt.IsZero() {
		startedAt = time.Now()
	}

	return &Service{
		appName:   appName,
		env:       env,
		version:   version,
		startedAt: startedAt.UTC(),
	}
}

// Snapshot returns a consistent status view at a specific time.
func (s *Service) Snapshot(now time.Time) Response {
	now = now.UTC()
	uptime := now.Sub(s.startedAt).Seconds()
	if uptime < 0 {
		uptime = 0
	}

	return Response{
		Status:        "ok",
		Service:       s.appName,
		Environment:   s.env,
		Version:       s.version,
		Timestamp:     now.Format(time.RFC3339Nano),
		UptimeSeconds: int64(uptime),
	}
}
