package sections

import "github.com/Silo-Server/silo-server/internal/catalog"

type FilterBuilder = catalog.QueryBuilder

func NewFilterBuilder(alias string) *catalog.QueryBuilder {
	return catalog.NewQueryBuilder(alias)
}
