package recommendations

import "strings"

// itemPeopleData holds extracted people data for reason computation.
type itemPeopleData struct {
	directors []string // person names
	actors    []string // top-5 actor names (by sort_order)
}

// computeReason determines the highest-priority reason two items are connected.
// Priority: shared_director > shared_actor > shared_studio > shared_genre > similar_content.
func computeReason(source, candidate *itemPeopleData, sourceStudios, candidateStudios, sourceGenres, candidateGenres []string) (reason, detail string) {
	// 1. Shared director.
	if source != nil && candidate != nil {
		for _, sd := range source.directors {
			for _, cd := range candidate.directors {
				if sd == cd {
					return "shared_director", sd
				}
			}
		}

		// 2. Shared actor.
		for _, sa := range source.actors {
			for _, ca := range candidate.actors {
				if sa == ca {
					return "shared_actor", sa
				}
			}
		}
	}

	// 3. Shared studio.
	for _, ss := range sourceStudios {
		for _, cs := range candidateStudios {
			if ss == cs {
				return "shared_studio", ss
			}
		}
	}

	// 4. Shared genre.
	for _, sg := range sourceGenres {
		for _, cg := range candidateGenres {
			if strings.EqualFold(sg, cg) {
				return "shared_genre", sg
			}
		}
	}

	// 5. Fallback.
	return "similar_content", ""
}
