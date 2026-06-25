package catalog

import (
	"context"
	"log/slog"
)

type CatalogSearchService struct {
	settings CatalogSearchSettings
	provider CatalogSearchProvider
	meili    *MeilisearchSearchProvider
	state    *SearchIndexEventRepository
}

func NewCatalogSearchService(
	ctx context.Context,
	settingsStore SettingsStore,
	itemRepo *ItemRepository,
	stateRepo *SearchIndexEventRepository,
) *CatalogSearchService {
	settings, err := LoadCatalogSearchSettings(ctx, settingsStore)
	if err != nil {
		slog.Warn("catalog search: failed to load settings; using postgres", "err", err)
		settings = DefaultCatalogSearchSettings()
	}
	return NewCatalogSearchServiceFromSettings(settings, itemRepo, stateRepo)
}

func NewCatalogSearchServiceFromSettings(
	settings CatalogSearchSettings,
	itemRepo *ItemRepository,
	stateRepo *SearchIndexEventRepository,
) *CatalogSearchService {
	fallback := NewPostgresSearchProvider(itemRepo)
	service := &CatalogSearchService{
		settings: settings,
		provider: fallback,
		state:    stateRepo,
	}
	if settings.Provider != SearchProviderMeilisearch {
		return service
	}
	if settings.MeilisearchURL == "" {
		slog.Warn("catalog search: meilisearch selected without URL; using postgres")
		return service
	}
	meili, err := NewMeilisearchSearchProvider(itemRepo, stateRepo, fallback, MeilisearchProviderConfig{
		URL:              settings.MeilisearchURL,
		APIKey:           settings.MeilisearchAPIKey,
		Index:            settings.MeilisearchIndex,
		Timeout:          settings.Timeout,
		MatchingStrategy: settings.MatchingStrategy,
	})
	if err != nil {
		slog.Warn("catalog search: failed to initialize meilisearch provider; using postgres", "err", err)
		return service
	}
	service.provider = meili
	service.meili = meili
	return service
}

func (s *CatalogSearchService) Provider() CatalogSearchProvider {
	if s == nil || s.provider == nil {
		return nil
	}
	return s.provider
}

func (s *CatalogSearchService) Status(ctx context.Context) CatalogSearchRuntimeStatus {
	settings := DefaultCatalogSearchSettings()
	if s != nil {
		settings = s.settings
	}
	status := CatalogSearchRuntimeStatus{
		ConfiguredProvider: settings.Provider,
		ActiveProvider:     SearchProviderPostgres,
		Meilisearch: CatalogSearchMeiliStatus{
			Configured:       settings.Provider == SearchProviderMeilisearch && settings.MeilisearchURL != "",
			Healthy:          false,
			CircuitState:     "not_configured",
			TimeoutMS:        int(settings.Timeout.Milliseconds()),
			MatchingStrategy: settings.MatchingStrategy,
		},
		Index: CatalogSearchIndexStateStatus{
			ExpectedSchemaVersion: SearchMeilisearchSchemaVersion,
		},
		Tasks: []CatalogSearchTaskLink{
			{Key: "sync_catalog_search_index", Name: "Sync Catalog Search Index", Href: "/admin/tasks/sync_catalog_search_index"},
			{Key: "rebuild_catalog_search_index", Name: "Rebuild Catalog Search Index", Href: "/admin/tasks/rebuild_catalog_search_index"},
		},
	}
	if s != nil {
		if _, ok := s.provider.(*MeilisearchSearchProvider); ok {
			status.ActiveProvider = SearchProviderMeilisearch
		}
		if s.meili != nil {
			status.Meilisearch = s.meili.Status()
		}
		if s.state != nil {
			state, err := s.state.GetState(ctx, SearchProviderMeilisearch)
			if err == nil {
				status.Index.ActiveIndexUID = state.ActiveIndexUID
				status.Index.SchemaVersion = state.SchemaVersion
				status.Index.DocumentCount = state.DocumentCount
				status.Index.LastRebuildAt = state.LastRebuildAt
				status.Index.LastSyncAt = state.LastSyncAt
				status.Index.LastProcessedEventID = state.LastProcessedEventID
			}
			if pending, err := s.state.PendingCount(ctx, SearchProviderMeilisearch); err == nil {
				status.Index.PendingEvents = pending
			}
		}
	}
	return status
}
