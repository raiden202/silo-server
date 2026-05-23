package pathscope

import (
	"path/filepath"
	"strings"
)

var likeEscaper = strings.NewReplacer(
	`\`, `\\`,
	`%`, `\%`,
	`_`, `\_`,
)

func PrefixLike(pathPrefix string) string {
	clean := filepath.Clean(pathPrefix)
	if clean == string(filepath.Separator) {
		return likeEscaper.Replace(clean) + "%"
	}
	return likeEscaper.Replace(clean) + string(filepath.Separator) + "%"
}
