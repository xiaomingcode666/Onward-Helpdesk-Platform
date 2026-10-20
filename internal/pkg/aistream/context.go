// Package aistream carries an optional public-answer observer through workflow
// execution without coupling the model runtime to a transport.
package aistream

import "context"

type handlerKey struct{}
type Handler func(string)

func WithHandler(ctx context.Context, handler Handler) context.Context {
	return context.WithValue(ctx, handlerKey{}, handler)
}

func Enabled(ctx context.Context) bool {
	handler, _ := ctx.Value(handlerKey{}).(Handler)
	return handler != nil
}

// Emit sends a cumulative text snapshot. Snapshots tolerate dropped or repeated
// transport frames and allow tool calls to clear provisional assistant text.
func Emit(ctx context.Context, text string) {
	if handler, _ := ctx.Value(handlerKey{}).(Handler); handler != nil {
		handler(text)
	}
}
