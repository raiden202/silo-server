package sections

import (
	"encoding/json"
	"fmt"
	"strings"
)

const GeneratedHomeLibraryRecentSource = "home_library_recent"

type generatedHomeLibraryRecentConfig struct {
	FilterLibraryID *int   `json:"filter_library_id"`
	GeneratedSource string `json:"generated_source"`
}

func GeneratedHomeLibraryRecentConfig(libraryID int) json.RawMessage {
	config, err := json.Marshal(generatedHomeLibraryRecentConfig{
		FilterLibraryID: &libraryID,
		GeneratedSource: GeneratedHomeLibraryRecentSource,
	})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return config
}

func GeneratedHomeLibraryRecentTitle(sectionType SectionType, libraryName string) string {
	switch sectionType {
	case SectionRecentlyAdded:
		return fmt.Sprintf("Recently Added in %s", libraryName)
	case SectionRecentlyReleased:
		return fmt.Sprintf("Recently Released in %s", libraryName)
	default:
		return string(sectionType)
	}
}

func ParseGeneratedHomeLibraryRecentConfig(config json.RawMessage) (int, bool) {
	var cfg generatedHomeLibraryRecentConfig
	if len(config) == 0 {
		return 0, false
	}
	if err := json.Unmarshal(config, &cfg); err != nil {
		return 0, false
	}
	if cfg.GeneratedSource != GeneratedHomeLibraryRecentSource || cfg.FilterLibraryID == nil || *cfg.FilterLibraryID <= 0 {
		return 0, false
	}
	return *cfg.FilterLibraryID, true
}

func IsGeneratedHomeLibraryRecentSection(s *PageSection, libraryID int) bool {
	if s == nil || s.Scope != "home" || s.LibraryID != nil {
		return false
	}
	if s.SectionType != SectionRecentlyAdded && s.SectionType != SectionRecentlyReleased {
		return false
	}
	id, ok := ParseGeneratedHomeLibraryRecentConfig(s.Config)
	return ok && id == libraryID
}

func ShouldSyncGeneratedHomeLibraryRecentTitle(s *PageSection, oldLibraryName string) bool {
	if s == nil {
		return false
	}
	expected := GeneratedHomeLibraryRecentTitle(s.SectionType, oldLibraryName)
	return strings.TrimSpace(s.Title) == expected
}
