package metadata

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Silo-Server/silo-server/internal/catalog"
	"github.com/Silo-Server/silo-server/internal/models"
)

var (
	ErrPersonNotFound         = errors.New("person not found")
	ErrPersonMetadataNotFound = errors.New("no person metadata found from any provider")
)

type personRefreshRepo interface {
	Get(ctx context.Context, id int64) (*models.Person, error)
	Update(ctx context.Context, person models.Person) error
	FindRefreshCandidates(ctx context.Context, staleAfter time.Duration, limit int) ([]int64, error)
}

type PersonRefreshService struct {
	pool           *pgxpool.Pool
	pluginResolver pluginMetadataResolver
	repo           personRefreshRepo
	imageCacher    ImageCacher
	imageResolver  interface {
		ResolveImageURL(ctx context.Context, path string, variant string) string
	}
}

func NewPersonRefreshService(
	pool *pgxpool.Pool,
	pluginResolver pluginMetadataResolver,
	repo *catalog.PersonRepository,
) *PersonRefreshService {
	return &PersonRefreshService{
		pool:           pool,
		pluginResolver: pluginResolver,
		repo:           repo,
	}
}

func (s *PersonRefreshService) SetImageCacher(cacher ImageCacher) {
	s.imageCacher = cacher
}

func (s *PersonRefreshService) SetImageResolver(resolver interface {
	ResolveImageURL(ctx context.Context, path string, variant string) string
}) {
	s.imageResolver = resolver
}

func (s *PersonRefreshService) RefreshPerson(ctx context.Context, id int64) (*models.Person, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("person refresh repository is not configured")
	}
	if s.pluginResolver == nil || s.pool == nil {
		return nil, fmt.Errorf("person refresh providers are not configured")
	}

	providers, err := resolveEnabledProviders(ctx, s.pluginResolver, s.pool)
	if err != nil {
		return nil, fmt.Errorf("resolve person providers: %w", err)
	}

	return s.refreshPersonWithProviders(ctx, id, providers)
}

func (s *PersonRefreshService) FindCandidates(
	ctx context.Context,
	staleAfter time.Duration,
	limit int,
) ([]int64, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("person refresh repository is not configured")
	}
	return s.repo.FindRefreshCandidates(ctx, staleAfter, limit)
}

func (s *PersonRefreshService) refreshPersonWithProviders(
	ctx context.Context,
	id int64,
	providers []Provider,
) (*models.Person, error) {
	person, err := s.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPersonNotFound
		}
		return nil, fmt.Errorf("load person %d: %w", id, err)
	}
	if person == nil {
		return nil, ErrPersonNotFound
	}

	accumulator := PersonDetailResult{
		ProviderIDs: copyMap(personProviderIDs(*person)),
	}
	photoProviderID := ""
	hasMetadata := false

	for _, provider := range providers {
		personProvider, ok := provider.(PersonProvider)
		if !ok {
			continue
		}

		result, err := personProvider.GetPersonDetail(ctx, PersonDetailRequest{
			ProviderIDs: accumulator.ProviderIDs,
			Language:    "en",
		})
		if err != nil {
			slog.Warn("person refresh: provider detail lookup failed",
				"provider", provider.Slug(),
				"person_id", id,
				"error", err,
			)
			continue
		}
		if result == nil {
			continue
		}

		hasMetadata = true
		if photoProviderID == "" && strings.TrimSpace(result.PhotoPath) != "" {
			photoProviderID = provider.Slug()
		}
		MergePersonDetail(result, &accumulator, MergeFillEmpty)
	}

	if !hasMetadata {
		return nil, ErrPersonMetadataNotFound
	}

	if cachedPath, thumbhash, err := s.cachePersonPhoto(ctx, *person, accumulator, photoProviderID); err != nil {
		slog.Warn("person refresh: photo cache failed",
			"person_id", id,
			"provider", photoProviderID,
			"error", err,
		)
	} else {
		accumulator.PhotoPath = cachedPath
		if thumbhash != "" {
			accumulator.PhotoThumbhash = thumbhash
		}
	}

	existingDetail := personToPersonDetailResult(*person)
	MergePersonDetail(&accumulator, &existingDetail, MergeReplaceUnlocked)
	accumulator = existingDetail

	refreshed, err := mergePersonIntoRecord(*person, accumulator)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, refreshed); err != nil {
		return nil, fmt.Errorf("update person %d: %w", id, err)
	}

	return &refreshed, nil
}

func (s *PersonRefreshService) cachePersonPhoto(
	ctx context.Context,
	person models.Person,
	detail PersonDetailResult,
	photoProviderID string,
) (string, string, error) {
	if s.imageCacher == nil {
		return detail.PhotoPath, detail.PhotoThumbhash, nil
	}
	if strings.TrimSpace(detail.PhotoPath) == "" || detail.PhotoPath == "-" {
		return detail.PhotoPath, detail.PhotoThumbhash, nil
	}
	if !strings.Contains(detail.PhotoPath, "://") {
		return detail.PhotoPath, detail.PhotoThumbhash, nil
	}

	downloadURL := detail.PhotoPath
	if !strings.HasPrefix(downloadURL, "http://") && !strings.HasPrefix(downloadURL, "https://") {
		if s.imageResolver == nil {
			return "", "", fmt.Errorf("plugin image resolver is not configured")
		}
		downloadURL = s.imageResolver.ResolveImageURL(ctx, detail.PhotoPath, "original")
		if downloadURL == "" {
			return "", "", fmt.Errorf("resolved empty URL for %q", detail.PhotoPath)
		}
	}

	providerID := photoProviderID
	if providerID == "" {
		providerID = primaryPersonProviderID(detail.ProviderIDs)
	}
	if providerID == "" {
		providerID = "unknown"
	}

	contentID := personCacheContentID(person, detail.ProviderIDs, providerID)
	result, err := s.imageCacher.CacheImage(ctx, CacheImageRequest{
		SourceURL:   downloadURL,
		ProviderID:  providerID,
		ContentType: "people",
		ContentID:   contentID,
		ImageType:   ImagePoster,
	})
	if err != nil {
		return "", "", err
	}

	return cachedOriginalImagePath(result.BasePath, result.Ext), result.Thumbhash, nil
}

func personProviderIDs(person models.Person) map[string]string {
	ids := map[string]string{}
	if person.TmdbID != "" {
		ids["tmdb"] = person.TmdbID
	}
	if person.ImdbID != "" {
		ids["imdb"] = person.ImdbID
	}
	if person.TvdbID != "" {
		ids["tvdb"] = person.TvdbID
	}
	if person.PlexGUID != "" {
		ids["plex"] = person.PlexGUID
	}
	return ids
}

func personToPersonDetailResult(person models.Person) PersonDetailResult {
	result := PersonDetailResult{
		Name:           person.Name,
		SortName:       person.SortName,
		Bio:            person.Bio,
		Birthplace:     person.Birthplace,
		Homepage:       person.Homepage,
		PhotoPath:      person.PhotoPath,
		PhotoThumbhash: person.PhotoThumbhash,
		ProviderIDs:    copyMap(personProviderIDs(person)),
	}
	if person.BirthDate != nil {
		result.BirthDate = person.BirthDate.Format("2006-01-02")
	}
	if person.DeathDate != nil {
		result.DeathDate = person.DeathDate.Format("2006-01-02")
	}
	return result
}

func mergePersonIntoRecord(person models.Person, detail PersonDetailResult) (models.Person, error) {
	birthDate, err := parseOptionalPersonDate(detail.BirthDate)
	if err != nil {
		return person, fmt.Errorf("parse birth date for person %d: %w", person.ID, err)
	}
	deathDate, err := parseOptionalPersonDate(detail.DeathDate)
	if err != nil {
		return person, fmt.Errorf("parse death date for person %d: %w", person.ID, err)
	}

	person.Name = detail.Name
	person.SortName = detail.SortName
	person.Bio = detail.Bio
	person.BirthDate = birthDate
	person.DeathDate = deathDate
	person.Birthplace = detail.Birthplace
	person.Homepage = detail.Homepage
	person.PhotoPath = detail.PhotoPath
	person.PhotoThumbhash = detail.PhotoThumbhash
	person.TmdbID = detail.ProviderIDs["tmdb"]
	person.ImdbID = detail.ProviderIDs["imdb"]
	person.TvdbID = detail.ProviderIDs["tvdb"]
	if plexID := detail.ProviderIDs["plex"]; plexID != "" {
		person.PlexGUID = plexID
	}

	return person, nil
}

func parseOptionalPersonDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func primaryPersonProviderID(providerIDs map[string]string) string {
	for _, key := range []string{"tmdb", "tvdb", "imdb", "metadb"} {
		if providerIDs[key] != "" {
			return key
		}
	}
	return ""
}

func personCacheContentID(
	person models.Person,
	providerIDs map[string]string,
	providerID string,
) string {
	if providerID != "" && providerIDs[providerID] != "" {
		return providerIDs[providerID]
	}
	for _, key := range []string{"tmdb", "tvdb", "imdb", "metadb"} {
		if providerIDs[key] != "" {
			return providerIDs[key]
		}
	}
	return strconv.FormatInt(person.ID, 10)
}

func cachedOriginalImagePath(basePath, ext string) string {
	if basePath == "" {
		return ""
	}
	if strings.Contains(basePath, "/original.") {
		return basePath
	}
	if ext == "" {
		ext = ".jpg"
	}
	return strings.TrimRight(basePath, "/") + "/original" + ext
}
