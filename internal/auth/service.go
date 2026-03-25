package auth

import (
	"context"
	"time"

	"refleks-api/internal/supabase"
)

// Service provides authentication-related operations.
type Service struct {
	supabaseClient *supabase.Client
}

// SessionStubResponse is a scaffold response for upcoming auth integration.
type SessionStubResponse struct {
	Status             string `json:"status"`
	Message            string `json:"message"`
	SupabaseConfigured bool   `json:"supabase_configured"`
	SupabaseReachable  bool   `json:"supabase_reachable"`
	Authenticated      bool   `json:"authenticated"`
}

// SteamLoginStubResponse is a scaffold for premium-site Steam login.
type SteamLoginStubResponse struct {
	Status            string `json:"status"`
	Message           string `json:"message"`
	Provider          string `json:"provider"`
	PremiumScope      string `json:"premium_scope"`
	Authenticated     bool   `json:"authenticated"`
	TokenIssued       bool   `json:"token_issued"`
	RequiresSteamOIDC bool   `json:"requires_steam_oidc"`
}

// NewService creates an auth service.
func NewService(supabaseClient *supabase.Client) *Service {
	return &Service{supabaseClient: supabaseClient}
}

// SessionStub returns a stable contract while auth is being implemented.
func (s *Service) SessionStub(ctx context.Context) SessionStubResponse {
	resp := SessionStubResponse{
		Status:             "stub",
		Message:            "auth integration pending",
		SupabaseConfigured: s.supabaseClient != nil,
		Authenticated:      false,
	}

	if s.supabaseClient == nil {
		return resp
	}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp.SupabaseReachable = s.supabaseClient.Ping(pingCtx) == nil
	return resp
}

// SteamLoginStub returns a stable contract for upcoming Steam auth integration.
func (s *Service) SteamLoginStub() SteamLoginStubResponse {
	return SteamLoginStubResponse{
		Status:            "stub",
		Message:           "steam login integration pending",
		Provider:          "steam",
		PremiumScope:      "parquet_lab_private",
		Authenticated:     false,
		TokenIssued:       false,
		RequiresSteamOIDC: true,
	}
}
