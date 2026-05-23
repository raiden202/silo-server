package catalog

import "strings"

// IsLiveQueryType returns true for collection types that derive membership
// from a query at read time rather than stored items.
func IsLiveQueryType(collectionType string) bool {
	return strings.EqualFold(strings.TrimSpace(collectionType), "smart")
}

// IsSyncableType returns true for collection types that support external sync.
func IsSyncableType(collectionType string) bool {
	switch strings.TrimSpace(strings.ToLower(collectionType)) {
	case "mdblist", "tmdb", "trakt":
		return true
	default:
		return false
	}
}
