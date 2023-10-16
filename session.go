package session

import (
	"context"

	"github.com/gofrs/uuid"

	"gitlab.com/gorib/try"
)

type SessionKey struct{}

type Session struct {
	Uri           string
	Method        string
	Body          string
	CorrelationId string
	UserAgent     string
	UserLanguage  string
	SourceIp      string
}

func SetupSession(session Session) *Session {
	if session.CorrelationId == "" {
		session.CorrelationId = try.Throw(uuid.NewV7()).String()
	}
	return &session
}

func NewContextWithSession(ctx context.Context, session Session) context.Context {
	ctx = context.WithValue(ctx, SessionKey{}, SetupSession(session))
	return ctx
}

func SessionFromContext(ctx context.Context) *Session {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(SessionKey{}).(*Session)
	return s
}

func NewEmptyContextWithSession(ctx context.Context) context.Context {
	session := SessionFromContext(ctx)
	if session == nil {
		session = SetupSession(Session{})
	}
	return NewContextWithSession(context.Background(), *session)
}
