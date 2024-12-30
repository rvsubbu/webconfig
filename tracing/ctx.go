package tracing

import "context"

type contextKey string

func (c contextKey) String() string {
	return string(c)
}

// SetContext - setting context for logging
func SetContext(ctx context.Context, ctxName string, ctxValue interface{}) context.Context {
	return context.WithValue(ctx, contextKey(ctxName), ctxValue)
}

// GetContext - getting context value from context
func GetContext(ctx context.Context, key string) interface{} {
	return ctx.Value(contextKey(key))
}
