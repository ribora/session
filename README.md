# gorib/session

Request and command identity, carried in the context. One struct that every
transport fills on the way in and every client reads on the way out, so a
correlation id and a trace context survive an HTTP call, a gRPC call, an AMQP
delivery and a CLI command without any of them knowing about the others.

Zero dependencies — the module requires nothing beyond the standard library
(`uuid` is stdlib as of Go 1.27).

## The struct

```go
type Session struct {
	Uri           string
	Method        string
	Body          string
	CorrelationId string
	UserAgent     string
	UserLanguage  string
	SourceIp      string

	Traceparent string
	Tracestate  string
}
```

`Method` is the transport, not the HTTP verb alone: `GET`/`POST` from HTTP,
`GRPC`, `AMQP`, `CLI`, `CRON`. `Uri` is whatever names the work in that
transport — a URL, a full gRPC method, a routing key, a command name. `Body`
is the raw payload, kept for error reporting.

`Traceparent` and `Tracestate` are [W3C Trace Context](https://www.w3.org/TR/trace-context/).
They are filled by transport middleware on entry and read by client-side
propagation on exit; the tracing middleware overwrites `Traceparent` with the
id of the span it just started, so an outgoing call is a child of the current
span rather than of the incoming one.

## API

| Function | What |
|---|---|
| `Setup(Session) *Session` | Fills a missing `CorrelationId` with a UUIDv7 and returns a pointer |
| `NewContextWithSession(ctx, Session) context.Context` | `Setup` plus store under the package key |
| `FromContext(ctx) *Session` | The session, or nil — a nil `ctx` is tolerated |
| `NewEmptyContextWithSession(ctx) context.Context` | A `context.Background()` carrying a copy of the current session |

## The pointer is deliberate

`FromContext` returns a `*Session`, and middleware mutates it in place —
that is how the tracing layer publishes the span it started to the client
layer without rebuilding the context. Two consequences worth knowing:

- everything downstream of the middleware sees the mutation, including code
  that captured the context earlier;
- the struct is not safe for concurrent mutation. In practice one goroutine
  owns a request, and fan-out uses `NewEmptyContextWithSession`, which copies.

## Detaching from the request

Work that must outlive the request — a goroutine started from a handler —
cannot keep the request context: its cancellation would kill the work when the
client disconnects. `NewEmptyContextWithSession` returns a context built on
`context.Background()` carrying a *copy* of the current session, so the
correlation id follows the work while the deadline does not.

```go
bg := session.NewEmptyContextWithSession(ctx)
go func() {
	if err := s.notify(bg, order); err != nil {
		logger.Error(err, pry.Ctx(bg))
	}
}()
```

If the incoming context has no session at all, a fresh one is created with a
new correlation id rather than returning nothing.

## Who fills it, who reads it

| Package | On the way in | On the way out |
|---|---|---|
| `http` | `middleware.NewSession` reads the correlation, user-agent, language, source-ip and traceparent headers | the same middleware echoes `CorrelationId` back as a response header; `client` sends all of them |
| `grpc` | `interceptor.Session` reads incoming metadata, falling back to `peer` for `SourceIp` and to gRPC's own `user-agent` | `client/interceptor.Session` appends them to outgoing metadata |
| `amqp` | the consumer builds a session per delivery from the message headers and `CorrelationId` | the publisher writes them back into `amqp.Publishing` headers |
| `waffle` | `app.NewContextWithCommand` (`CLI`) and the cron tool (`CRON`) synthesize one, so a background job is as traceable as a request | — |
| `pry` | — | `pry.Ctx(ctx)` forwards the fields to Sentry as tags, request url/method/data and headers |

Header names are not this package's business: each transport owns its own
constants (`http/model`, `grpc/model`, `amqp/model`), because the spellings
differ — `correlation-id` in gRPC metadata, `User-Agent` in AMQP headers.

## What it does not do

- No propagation of its own. It is a data holder; the transports move it.
- No trace-context parsing. `Traceparent` is an opaque string here; `hilton`
  formats and parses it.
- No authentication or user identity. Nothing in `Session` is a claim about
  who the caller is — it identifies a request, not a principal.
