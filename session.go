package session

import (
	"context"
	"math/rand"
	"strconv"

	"github.com/gofrs/uuid"
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
		if correlation, err := uuid.NewV7(); err == nil {
			session.CorrelationId = correlation.String()
		} else {
			session.CorrelationId = strconv.Itoa(int(rand.Uint32()))
		}
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
