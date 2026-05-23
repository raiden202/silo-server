package trakt

// CollectionEntry is a normalized Trakt collection/discovery result.
type CollectionEntry struct {
	TraktID   int
	TMDBID    int
	TVDBID    int
	IMDbID    string
	MediaType string
	Title     string
	Year      int
	Rank      int
}
