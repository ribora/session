package session

import (
	"context"
	"uuid"
)

type Key struct{}

// Session is shared by pointer across every goroutine holding a request's
// context. Every field is written once, by the transport middleware that
// builds the session, and is read-only from then on — which is what keeps the
// struct safe to copy. The W3C Trace Context is rewritten per span and so does
// not live here; it rides the context itself.
type Session struct {
	Uri           string
	Method        string
	Body          string
	CorrelationId string
	UserAgent     string
	UserLanguage  string
	SourceIp      string
}

func Setup(session Session) *Session {
	if session.CorrelationId == "" {
		session.CorrelationId = uuid.NewV7().String()
	}
	return &session
}

func NewContextWithSession(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, Key{}, Setup(session))
}

func FromContext(ctx context.Context) *Session {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(Key{}).(*Session)
	return s
}

func NewEmptyContextWithSession(ctx context.Context) context.Context {
	session := FromContext(ctx)
	if session == nil {
		session = Setup(Session{})
	}
	detached := NewContextWithSession(context.Background(), *session)
	traceparent, tracestate := TraceFromContext(ctx)
	if traceparent == "" && tracestate == "" {
		return detached
	}
	return WithTrace(detached, traceparent, tracestate)
}

type traceKey struct{}

// traceContext is the W3C Trace Context (https://www.w3.org/TR/trace-context/).
// It is immutable: a write derives a new context instead of overwriting what
// the old one holds, so a span's traceparent is visible for exactly as long as
// that span's context is, and never to the parent that outlives the span.
type traceContext struct {
	traceparent string
	tracestate  string
}

func WithTrace(ctx context.Context, traceparent, tracestate string) context.Context {
	return context.WithValue(ctx, traceKey{}, traceContext{
		traceparent: traceparent,
		tracestate:  tracestate,
	})
}

func WithTraceparent(ctx context.Context, traceparent string) context.Context {
	_, tracestate := TraceFromContext(ctx)
	return WithTrace(ctx, traceparent, tracestate)
}

func WithTracestate(ctx context.Context, tracestate string) context.Context {
	traceparent, _ := TraceFromContext(ctx)
	return WithTrace(ctx, traceparent, tracestate)
}

func TraceFromContext(ctx context.Context) (traceparent, tracestate string) {
	if ctx == nil {
		return "", ""
	}
	t, _ := ctx.Value(traceKey{}).(traceContext)
	return t.traceparent, t.tracestate
}

func TraceparentFromContext(ctx context.Context) string {
	traceparent, _ := TraceFromContext(ctx)
	return traceparent
}

func TracestateFromContext(ctx context.Context) string {
	_, tracestate := TraceFromContext(ctx)
	return tracestate
}
