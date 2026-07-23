package config

import "testing"

func TestEffectiveAdminSettingsUsesRuntimeDefaults(t *testing.T) {
	effective := EffectiveAdminSettings(map[string]string{
		"database.max_connections": "",
		"server.log_level":         "debug",
		"custom.setting":           "kept",
	})

	if got := effective["database.max_connections"]; got != "20" {
		t.Fatalf("database.max_connections = %q, want 20", got)
	}
	if got := effective["s3.public_path_style"]; got != "true" {
		t.Fatalf("s3.public_path_style = %q, want true", got)
	}
	if got := effective["playback.transcode_enabled"]; got != "true" {
		t.Fatalf("playback.transcode_enabled = %q, want true", got)
	}
	if got := effective["theme.catalog_url"]; got != DefaultThemeCatalogURL {
		t.Fatalf("theme.catalog_url = %q, want %q", got, DefaultThemeCatalogURL)
	}
	if got := effective["server.log_level"]; got != "debug" {
		t.Fatalf("server.log_level = %q, want debug", got)
	}
	if got := effective["custom.setting"]; got != "kept" {
		t.Fatalf("custom.setting = %q, want kept", got)
	}
}

func TestEffectiveAdminSettingsUsesLegacyS3FallbacksBeforeDefaults(t *testing.T) {
	effective := EffectiveAdminSettings(map[string]string{
		"s3.operational_path_style": "false",
		"s3.operational_token_ttl":  "3600",
	})

	if got := effective["s3.public_path_style"]; got != "false" {
		t.Fatalf("s3.public_path_style = %q, want legacy false", got)
	}
	if got := effective["s3.private_path_style"]; got != "false" {
		t.Fatalf("s3.private_path_style = %q, want legacy false", got)
	}
	if got := effective["s3.public_token_ttl"]; got != "3600" {
		t.Fatalf("s3.public_token_ttl = %q, want legacy 3600", got)
	}
}

func TestEffectiveAdminSettingsCanonicalS3ValuesOverrideLegacyFallbacks(t *testing.T) {
	effective := EffectiveAdminSettings(map[string]string{
		"s3.public_path_style":      "true",
		"s3.private_path_style":     "false",
		"s3.operational_path_style": "true",
		"s3.public_token_ttl":       "7200",
		"s3.operational_token_ttl":  "3600",
	})

	if got := effective["s3.public_path_style"]; got != "true" {
		t.Fatalf("s3.public_path_style = %q, want canonical true", got)
	}
	if got := effective["s3.private_path_style"]; got != "false" {
		t.Fatalf("s3.private_path_style = %q, want canonical false", got)
	}
	if got := effective["s3.public_token_ttl"]; got != "7200" {
		t.Fatalf("s3.public_token_ttl = %q, want canonical 7200", got)
	}
}

func TestEffectiveAdminSettingsEmptyCanonicalS3ValuesUseLegacyFallbacks(t *testing.T) {
	effective := EffectiveAdminSettings(map[string]string{
		"s3.public_path_style":      "",
		"s3.private_path_style":     "",
		"s3.operational_path_style": "false",
		"s3.public_token_ttl":       "",
		"s3.operational_token_ttl":  "3600",
	})

	if got := effective["s3.public_path_style"]; got != "false" {
		t.Fatalf("s3.public_path_style = %q, want legacy false", got)
	}
	if got := effective["s3.private_path_style"]; got != "false" {
		t.Fatalf("s3.private_path_style = %q, want legacy false", got)
	}
	if got := effective["s3.public_token_ttl"]; got != "3600" {
		t.Fatalf("s3.public_token_ttl = %q, want legacy 3600", got)
	}
}

func TestNormalizeAdminSettingRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{key: "database.max_connections", value: "0"},
		{key: "metadata.cache_images", value: "maybe"},
		{key: "auth.access_token_expiry", value: "forever"},
		{key: "recommendations.embeddings_cron", value: "not a cron"},
		{key: "notifications.server_channels.batch_seconds", value: "119"},
		{key: "catalog.search.meilisearch.semantic_ratio", value: "1.2"},
		{key: "email.smtp_port", value: "70000"},
		{key: "theme.catalog_url", value: "http://raw.githubusercontent.com/Silo-Server/silo-themes/main/catalog.json"},
		{key: "theme.catalog_url", value: "https://example.com/catalog.json"},
		{key: "redis.url", value: "not-a-url"},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			if _, err := NormalizeAdminSetting(tc.key, tc.value); err == nil {
				t.Fatalf("NormalizeAdminSetting(%q, %q) returned nil error", tc.key, tc.value)
			}
		})
	}
}

func TestNormalizeAdminSettingAcceptsApprovedThemeCatalogURL(t *testing.T) {
	got, err := NormalizeAdminSetting(
		"theme.catalog_url",
		"https://raw.githubusercontent.com/Silo-Server/silo-themes/main/catalog.json/",
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://raw.githubusercontent.com/Silo-Server/silo-themes/main/catalog.json" {
		t.Fatalf("normalized URL = %q", got)
	}
}

func TestValidateAdminSettingsChecksProspectiveRelationships(t *testing.T) {
	values := map[string]string{
		"auth.access_token_expiry":      "48h",
		"auth.refresh_token_expiry":     "24h",
		"playback.watched_threshold":    "90",
		"playback.min_resume_threshold": "5",
	}
	if err := ValidateAdminSettings(values); err == nil {
		t.Fatal("ValidateAdminSettings() returned nil for refresh shorter than access")
	}

	values["auth.refresh_token_expiry"] = "72h"
	values["playback.min_resume_threshold"] = "95"
	if err := ValidateAdminSettings(values); err == nil {
		t.Fatal("ValidateAdminSettings() returned nil for resume threshold above watched")
	}
}

func TestValidateAdminSettingsRequiresDurableRedisTransport(t *testing.T) {
	values := map[string]string{"ratelimit.backend": "redis"}
	if err := ValidateAdminSettings(values); err == nil {
		t.Fatal("ValidateAdminSettings() accepted Redis backend without a durable transport")
	}

	if err := ValidateAdminSettingsWithCapabilities(values, AdminSettingsCapabilities{
		RedisBootstrapAvailable: true,
	}); err != nil {
		t.Fatalf("bootstrap Sentinel transport was rejected: %v", err)
	}

	values["redis.url"] = "redis://cache.example.invalid:6379"
	if err := ValidateAdminSettings(values); err != nil {
		t.Fatalf("persisted Redis URL was rejected: %v", err)
	}

	values["redis.url"] = "not-a-url"
	if err := ValidateAdminSettings(values); err == nil {
		t.Fatal("ValidateAdminSettings() accepted a malformed Redis URL")
	}
}

func TestNormalizeAdminSettingCanonicalizesRedisURL(t *testing.T) {
	got, err := NormalizeAdminSetting("redis.url", "  rediss://cache.example.invalid:6380/2  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "rediss://cache.example.invalid:6380/2" {
		t.Fatalf("normalized Redis URL = %q", got)
	}
}
