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
}
```

`Method` is the transport, not the HTTP verb alone: `GET`/`POST` from HTTP,
`GRPC`, `AMQP`, `OUTBOX`, `CLI`, `CRON`. `Uri` is whatever names the work in that
transport — a URL, a full gRPC method, a routing key, a command name. `Body`
is the raw payload, kept for error reporting; an AMQP delivery handled in a
batch of several carries none.

Every field is written once, by the transport that builds the session, and is
read-only from then on. That is what makes the struct safe to copy out of the
context, which `pry` does on every log line.

## The trace context

[W3C Trace Context](https://www.w3.org/TR/trace-context/) does not live in the
struct. The tracing middleware replaces the traceparent with the id of the span
it just started, so an outgoing call is a child of the current span rather than
of the incoming one — and that happens while other goroutines hold the same
session. So it rides the context instead, where a write derives rather than
mutates:

```go
ctx, span := tracer.Start(ctx, name, ...)
if tp := hilton.FormatTraceparent(span.SpanContext()); tp != "" {
	ctx = session.WithTraceparent(ctx, tp)
}
// the derived ctx is what must reach the handler
```

```go
traceparent, tracestate := session.TraceFromContext(ctx)
```

`WithTraceparent` and `WithTracestate` each keep the other half. Reading a
context that never passed through tracing middleware yields empty strings
rather than panicking, and a nil `ctx` is tolerated.

## Scope follows the span

The traceparent is visible for exactly as long as the context it was written
to. A function that starts a span, uses it and returns cannot leave its
traceparent behind for its caller:

```go
func (r *publisher) Publish(ctx context.Context, ...) error {
	ctx, span := tracer.Start(ctx, r.exchange+" publish", ...)
	defer span.End()
	ctx = session.WithTraceparent(ctx, tp)
	...
}
// the caller's ctx still names the request span, not the finished publish span
```

That is the whole reason the pair is not a field. A writer must therefore make
sure the derived context is the one that travels on — `src.SetRequest`, the
handler call, `msgs[i].WithContext` — or the write is lost.

## API

| Function | What |
|---|---|
| `Setup(Session) *Session` | Fills a missing `CorrelationId` with a UUIDv7 and returns a pointer |
| `NewContextWithSession(ctx, Session) context.Context` | `Setup` plus store under the package key |
| `FromContext(ctx) *Session` | The session, or nil — a nil `ctx` is tolerated |
| `NewEmptyContextWithSession(ctx) context.Context` | A `context.Background()` carrying a copy of the current session and its trace |
| `WithTrace(ctx, traceparent, tracestate) context.Context` | Both halves |
| `WithTraceparent(ctx, traceparent) / WithTracestate(ctx, tracestate)` | One half, keeping the other |
| `TraceFromContext(ctx) (traceparent, tracestate string)` | The trace, or empty strings |
| `TraceparentFromContext(ctx) / TracestateFromContext(ctx) string` | One half |

## The pointer is deliberate

`FromContext` returns a `*Session` so that reading it costs one context lookup
and no allocation — `pry.Ctx` runs on every log line. Nothing mutates it: the
one thing that used to be rewritten in place now lives on the context. Writing
an identity field after the session reaches a context is a data race the
compiler will not catch.

## Detaching from the request

Work that must outlive the request — a goroutine started from a handler —
cannot keep the request context: its cancellation would kill the work when the
client disconnects. `NewEmptyContextWithSession` returns a context built on
`context.Background()` carrying a *copy* of the current session, so the
correlation id follows the work while the deadline does not. The trace is
carried over explicitly — `context.Background()` holds nothing — so the
detached work starts under the span that forked it.

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
| `waffle` | `app.NewContextWithCommand` (`CLI`), the cron tool (`CRON`) and the outbox relay (`OUTBOX`) synthesize one, so a background job is as traceable as a request | — |
| `pry` | — | `pry.Ctx(ctx)` forwards the fields to Sentry as tags, request url/method/data and headers |

Header names are not this package's business: each transport owns its own
constants (`http/model`, `grpc/model`, `amqp/model`), because the spellings
differ — `correlation-id` in gRPC metadata, `User-Agent` in AMQP headers.

## What it does not do

- No propagation of its own. It is a holder; the transports move it.
- No trace-context parsing. The traceparent is an opaque string here; `hilton`
  formats and parses it.
- No authentication or user identity. Nothing in `Session` is a claim about
  who the caller is — it identifies a request, not a principal.
