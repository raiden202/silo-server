package catalog

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Silo-Server/silo-server/internal/models"
)

const (
	SearchProviderPostgres    = "postgres"
	SearchProviderMeilisearch = "meilisearch"

	SearchSettingProvider                    = "catalog.search.provider"
	SearchSettingMeilisearchURL              = "catalog.search.meilisearch.url"
	SearchSettingMeilisearchAPIKey           = "catalog.search.meilisearch.api_key"
	SearchSettingMeilisearchIndex            = "catalog.search.meilisearch.index"
	SearchSettingMeilisearchTimeoutMS        = "catalog.search.meilisearch.timeout_ms"
	SearchSettingMeilisearchMatchingStrategy = "catalog.search.meilisearch.matching_strategy"

	DefaultMeilisearchIndex            = "silo_media_items"
	DefaultMeilisearchTimeoutMS        = 800
	DefaultMeilisearchMatchingStrategy = "last"
	SearchMeilisearchSchemaVersion     = 1
)

var ErrSearchProviderFallback = errors.New("catalog search provider fallback")

type CatalogSearchRequest struct {
	Query     string
	ItemTypes []string
	Limit     int
	Offset    int
	Access    AccessFilter
}

type CatalogSearchResult struct {
	Items          []*models.MediaItem
	Total          int
	HasMore        bool
	TotalExact     bool
	Provider       string
	FallbackReason string
}

type CatalogSearchProvider interface {
	Search(ctx context.Context, req CatalogSearchRequest) (*CatalogSearchResult, error)
}

type PostgresSearchProvider struct {
	itemRepo *ItemRepository
}

func NewPostgresSearchProvider(itemRepo *ItemRepository) *PostgresSearchProvider {
	return &PostgresSearchProvider{itemRepo: itemRepo}
}

func (p *PostgresSearchProvider) Search(ctx context.Context, req CatalogSearchRequest) (*CatalogSearchResult, error) {
	if p == nil || p.itemRepo == nil {
		return nil, fmt.Errorf("postgres search provider requires item repository")
	}
	items, total, err := p.itemRepo.Search(ctx, req.Query, req.ItemTypes, req.Limit, req.Offset, req.Access)
	if err != nil {
		return nil, err
	}
	return &CatalogSearchResult{
		Items:      items,
		Total:      total,
		HasMore:    total > req.Offset+len(items),
		TotalExact: true,
		Provider:   SearchProviderPostgres,
	}, nil
}

type CatalogSearchSettings struct {
	Provider          string
	MeilisearchURL    string
	MeilisearchAPIKey string
	MeilisearchIndex  string
	Timeout           time.Duration
	MatchingStrategy  string
}

func DefaultCatalogSearchSettings() CatalogSearchSettings {
	return CatalogSearchSettings{
		Provider:         SearchProviderPostgres,
		MeilisearchIndex: DefaultMeilisearchIndex,
		Timeout:          time.Duration(DefaultMeilisearchTimeoutMS) * time.Millisecond,
		MatchingStrategy: DefaultMeilisearchMatchingStrategy,
	}
}

func LoadCatalogSearchSettings(ctx context.Context, store SettingsStore) (CatalogSearchSettings, error) {
	settings := DefaultCatalogSearchSettings()
	if store == nil {
		return settings, nil
	}
	values, err := store.GetAll(ctx)
	if err != nil {
		return settings, err
	}
	return CatalogSearchSettingsFromMap(values)
}

func CatalogSearchSettingsFromMap(values map[string]string) (CatalogSearchSettings, error) {
	settings := DefaultCatalogSearchSettings()
	if values == nil {
		return settings, nil
	}
	settings.Provider = normalizeCatalogSearchProvider(values[SearchSettingProvider])
	if settings.Provider == "" {
		return settings, fmt.Errorf("%s must be postgres or meilisearch", SearchSettingProvider)
	}
	settings.MeilisearchURL = strings.TrimSpace(values[SearchSettingMeilisearchURL])
	settings.MeilisearchAPIKey = strings.TrimSpace(values[SearchSettingMeilisearchAPIKey])
	if index := strings.TrimSpace(values[SearchSettingMeilisearchIndex]); index != "" {
		settings.MeilisearchIndex = index
	}
	if raw := strings.TrimSpace(values[SearchSettingMeilisearchTimeoutMS]); raw != "" {
		timeoutMS, err := strconv.Atoi(raw)
		if err != nil || timeoutMS <= 0 {
			return settings, fmt.Errorf("%s must be an integer greater than 0", SearchSettingMeilisearchTimeoutMS)
		}
		settings.Timeout = time.Duration(timeoutMS) * time.Millisecond
	}
	if strategy := strings.TrimSpace(strings.ToLower(values[SearchSettingMeilisearchMatchingStrategy])); strategy != "" {
		if strategy != "last" && strategy != "all" {
			return settings, fmt.Errorf("%s must be last or all", SearchSettingMeilisearchMatchingStrategy)
		}
		settings.MatchingStrategy = strategy
	}
	return settings, nil
}

func normalizeCatalogSearchProvider(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", SearchProviderPostgres:
		return SearchProviderPostgres
	case SearchProviderMeilisearch:
		return SearchProviderMeilisearch
	default:
		return ""
	}
}

type CatalogSearchRuntimeStatus struct {
	ConfiguredProvider string                        `json:"configured_provider"`
	ActiveProvider     string                        `json:"active_provider"`
	Meilisearch        CatalogSearchMeiliStatus      `json:"meilisearch"`
	Index              CatalogSearchIndexStateStatus `json:"index"`
	Tasks              []CatalogSearchTaskLink       `json:"tasks"`
}

type CatalogSearchMeiliStatus struct {
	Configured       bool       `json:"configured"`
	Healthy          bool       `json:"healthy"`
	CircuitState     string     `json:"circuit_state"`
	CircuitReason    string     `json:"circuit_reason,omitempty"`
	CircuitUntil     *time.Time `json:"circuit_until,omitempty"`
	LastFallback     string     `json:"last_fallback,omitempty"`
	TimeoutMS        int        `json:"timeout_ms"`
	MatchingStrategy string     `json:"matching_strategy"`
}

type CatalogSearchIndexStateStatus struct {
	ActiveIndexUID        string     `json:"active_index_uid"`
	SchemaVersion         int        `json:"schema_version"`
	ExpectedSchemaVersion int        `json:"expected_schema_version"`
	DocumentCount         int        `json:"document_count"`
	PendingEvents         int        `json:"pending_events"`
	LastRebuildAt         *time.Time `json:"last_rebuild_at,omitempty"`
	LastSyncAt            *time.Time `json:"last_sync_at,omitempty"`
	LastProcessedEventID  int64      `json:"last_processed_event_id"`
}

type CatalogSearchTaskLink struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Href string `json:"href"`
}

type CatalogSearchStatusProvider interface {
	Status(ctx context.Context) CatalogSearchRuntimeStatus
}
