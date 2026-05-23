package access

import "context"

type contextKey string

const scopeKey contextKey = "access_scope"

// SetScope stores a resolved access scope in context.
func SetScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeKey, scope)
}

// GetScope retrieves the resolved access scope from context.
func GetScope(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeKey).(Scope)
	return scope, ok
}
