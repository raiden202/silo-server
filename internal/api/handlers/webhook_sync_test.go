package handlers

import (
	"testing"
	"time"

	"github.com/Silo-Server/silo-server/internal/webhooksync"
)

func TestRequestWebhookURLWithPrefix(t *testing.T) {
	t.Parallel()

	if got := requestWebhookURLWithPrefix("https://example.com/", legacyPlexSyncPathPrefix, "secret"); got != "https://example.com/api/v1/plex-sync/webhooks/secret" {
		t.Fatalf("unexpected legacy webhook URL: %q", got)
	}
	if got := requestWebhookURLWithPrefix("https://example.com/", webhookSyncPathPrefix, "secret"); got != "https://example.com/api/v1/webhook-sync/webhooks/secret" {
		t.Fatalf("unexpected generic webhook URL: %q", got)
	}
}

func TestToLegacyPlexActorsResponse(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	profileID := "profile-1"
	resp := toLegacyPlexActorsResponse(&webhooksync.ActorMappingsResponse{
		Mappings: []webhooksync.ActorMapping{
			{
				ID:                11,
				ConnectionID:      "conn-1",
				ExternalActorID:   "42",
				ExternalActorName: "Alice",
				SiloProfileID:     &profileID,
				CreatedAt:         now,
				UpdatedAt:         now,
			},
		},
		DiscoveredActors: []webhooksync.DiscoveredActor{
			{ExternalActorID: "42", ExternalActorName: "Alice"},
			{ExternalActorID: "77", ExternalActorName: "Bob"},
		},
		AccountDiscoveryAvailable: true,
	})

	if !resp.AccountDiscoveryAvailable {
		t.Fatalf("expected discovery to be available")
	}
	if len(resp.Mappings) != 1 || resp.Mappings[0].PlexAccountID != 42 || resp.Mappings[0].SiloProfileID != "profile-1" {
		t.Fatalf("unexpected legacy mappings: %#v", resp.Mappings)
	}
	if len(resp.DiscoveredActors) != 2 || resp.DiscoveredActors[1].PlexAccountID != 77 {
		t.Fatalf("unexpected legacy discovered actors: %#v", resp.DiscoveredActors)
	}
}
