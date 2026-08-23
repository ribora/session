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

	// W3C Trace Context (https://www.w3.org/TR/trace-context/) — filled by
	// transport-layer middleware on entry, read by client-side propagation
	// on exit.
	Traceparent string
	Tracestate  string
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
	return NewContextWithSession(context.Background(), *session)
}
