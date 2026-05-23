package sections

import "testing"

func TestRecommendationTypesAreValid(t *testing.T) {
	for _, typ := range []SectionType{SectionRecommendedForYou, SectionBecauseYouWatched, SectionSimilarUsersLiked, SectionTasteMatch, SectionNextUp} {
		if !ValidSectionTypes[typ] {
			t.Errorf("ValidSectionTypes missing %q", typ)
		}
	}
}
