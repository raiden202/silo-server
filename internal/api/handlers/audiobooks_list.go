package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Silo-Server/silo-server/internal/catalog"
	"github.com/Silo-Server/silo-server/internal/models"
	"github.com/Silo-Server/silo-server/internal/recommendations"
	"github.com/Silo-Server/silo-server/internal/scanner"
	"github.com/Silo-Server/silo-server/internal/userstore"
)

// AudiobookHandler serves silo-native audiobook endpoints under
// /api/v1/audiobooks/*. Sub-plan 3 implements list/detail/progress;
// sub-plan 4's ABS-compat layer is separate.
type AudiobookHandler struct {
	Items         *catalog.ItemRepository
	Files         *scanner.FileRepository
	Catalog       *catalog.CatalogResolver
	StoreProvider userstore.UserStoreProvider
	Detail        *catalog.DetailService
	// Recs is optional. When set, the detail handler uses embedding-based
	// nearest-neighbor search for the "You might also like" rail and falls
	// back to shared-genre matching only when no embedding exists for the
	// source book yet.
	Recs *recommendations.Repo
}

// HandleListAudiobooks serves GET /api/v1/audiobooks?limit=&offset=.
// Paginated catalog of media_items with type='audiobook' scoped to the
// caller's accessible libraries.
func (h *AudiobookHandler) HandleListAudiobooks(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Catalog == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "audiobook catalog is not configured")
		return
	}

	req, err := audiobookCatalogRequest(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	result, err := h.Catalog.Resolve(r.Context(), req, requestAccessFilter(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "list audiobooks failed")
		return
	}

	items := h.audiobookListItems(r.Context(), result.Items)
	writeAudiobookListResponse(w, items, result.Total, req.Limit, req.Offset)
}

func audiobookCatalogRequest(values url.Values) (catalog.CatalogRequest, error) {
	catalogValues := make(url.Values, len(values)+1)
	for key, value := range values {
		catalogValues[key] = append([]string(nil), value...)
	}
	catalogValues.Set("type", "audiobook")
	return catalog.ParseCatalogRequest(catalogValues)
}

func (h *AudiobookHandler) audiobookListItems(ctx context.Context, items []*models.MediaItem) []audiobookListItem {
	out := make([]audiobookListItem, 0, len(items))
	stats := h.audiobookListStats(ctx, items)
	for _, item := range items {
		if item == nil {
			continue
		}
		resp := audiobookListItem{
			ContentID: item.ContentID,
			Title:     item.Title,
			Year:      item.Year,
			PosterURL: h.presignAudiobookPoster(ctx, item.PosterPath),
		}
		if stat, ok := stats[item.ContentID]; ok {
			resp.DurationSeconds = stat.DurationSeconds
			resp.Container = stat.Container
			resp.CodecAudio = stat.CodecAudio
			resp.FileCount = stat.FileCount
		}
		out = append(out, resp)
	}
	return out
}

type audiobookListStat struct {
	DurationSeconds int
	Container       string
	CodecAudio      string
	FileCount       int
}

func (h *AudiobookHandler) audiobookListStats(ctx context.Context, items []*models.MediaItem) map[string]audiobookListStat {
	if h == nil || h.Files == nil || len(items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item != nil {
			ids = append(ids, item.ContentID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := h.Files.Pool().Query(ctx, `
		SELECT
			content_id,
			COALESCE(SUM(duration), 0)::int AS duration_seconds,
			COALESCE(MIN(container), '') AS container,
			COALESCE(MIN(codec_audio), '') AS codec_audio,
			COUNT(id) AS file_count
		FROM media_files
		WHERE content_id = ANY($1)
		GROUP BY content_id
	`, ids)
	if err != nil {
		return nil
	}
	defer rows.Close()

	stats := make(map[string]audiobookListStat, len(ids))
	for rows.Next() {
		var contentID string
		var stat audiobookListStat
		if err := rows.Scan(&contentID, &stat.DurationSeconds, &stat.Container, &stat.CodecAudio, &stat.FileCount); err != nil {
			return nil
		}
		stats[contentID] = stat
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return stats
}

func audiobookListConditions(access catalog.AccessFilter, genre string) ([]string, []any, int, bool) {
	conditions := []string{"mi.type = 'audiobook'"}
	var args []any
	argIdx := 1

	if genre != "" {
		conditions = append(conditions, fmt.Sprintf("$%d = ANY(mi.genres)", argIdx))
		args = append(args, genre)
		argIdx++
	}
	if access.AllowedLibraryIDs != nil {
		if len(access.AllowedLibraryIDs) == 0 {
			return nil, nil, 1, true
		}
		conditions = append(conditions, fmt.Sprintf(`
			EXISTS (
				SELECT 1 FROM media_item_libraries mil
				WHERE mil.content_id = mi.content_id
				  AND mil.media_folder_id = ANY($%d)
			)`, argIdx))
		args = append(args, access.AllowedLibraryIDs)
		argIdx++
	}
	if len(access.DisabledLibraryIDs) > 0 {
		if access.AllowedLibraryIDs == nil {
			conditions = append(conditions, `
				EXISTS (
					SELECT 1 FROM media_item_libraries mil
					WHERE mil.content_id = mi.content_id
				)`)
		}
		conditions = append(conditions, fmt.Sprintf(`
			NOT EXISTS (
				SELECT 1 FROM media_item_libraries mil
				WHERE mil.content_id = mi.content_id
				  AND mil.media_folder_id = ANY($%d)
			)`, argIdx))
		args = append(args, access.DisabledLibraryIDs)
		argIdx++
	}
	catalog.ApplySectionAccessFilter("mi", catalog.AccessFilter{MaxContentRating: access.MaxContentRating}, &conditions, &args, &argIdx)
	return conditions, args, argIdx, false
}

func writeAudiobookListResponse(w http.ResponseWriter, items []audiobookListItem, total, limit, offset int) {
	if items == nil {
		items = []audiobookListItem{}
	}
	resp := struct {
		Items  []audiobookListItem `json:"items"`
		Total  int                 `json:"total"`
		Limit  int                 `json:"limit"`
		Offset int                 `json:"offset"`
	}{Items: items, Total: total, Limit: limit, Offset: offset}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type audiobookListItem struct {
	ContentID       string `json:"content_id"`
	Title           string `json:"title"`
	Year            int    `json:"year"`
	PosterURL       string `json:"poster_url,omitempty"`
	DurationSeconds int    `json:"duration_seconds"`
	Container       string `json:"container,omitempty"`
	CodecAudio      string `json:"codec_audio,omitempty"`
	FileCount       int    `json:"file_count"`
}

func (h *AudiobookHandler) presignAudiobookPoster(ctx context.Context, path string) string {
	if h == nil || h.Detail == nil || path == "" {
		return ""
	}
	return h.Detail.PresignURL(ctx, cardThumbnailPath(path), "card")
}
