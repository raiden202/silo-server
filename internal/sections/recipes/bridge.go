package recipes

import "errors"

// DelegateFunc is the bridge function package sections installs at startup.
// Avoids an import cycle: recipes describes "what", sections implements "how".
type DelegateFunc func(typ string, rc ResolverContext) (ResolvedItems, error)

var delegate DelegateFunc

// SetDelegate installs the resolver bridge. Called once from package sections at startup.
func SetDelegate(f DelegateFunc) { delegate = f }

// ErrNoDelegate is returned if a recipe is resolved before SetDelegate has run.
var ErrNoDelegate = errors.New("recipes: resolver delegate not installed")

func delegateResolve(typ string, rc ResolverContext) (ResolvedItems, error) {
	if delegate == nil {
		return ResolvedItems{}, ErrNoDelegate
	}
	return delegate(typ, rc)
}
