package session

import (
	"context"
	"uuid"
)

type Key struct{}

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
