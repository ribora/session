package session

import (
	"context"

	"github.com/gofrs/uuid"

	"gitlab.com/gorib/try"
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
		session.CorrelationId = try.Throw(uuid.NewV7()).String()
	}
	return &session
}

func NewContextWithSession(ctx context.Context, session Session) context.Context {
	ctx = context.WithValue(ctx, Key{}, Setup(session))
	return ctx
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
