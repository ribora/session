package session_test

import (
	"context"
	"reflect"
	"slices"
	"sync"
	"testing"

	"gitlab.com/gorib/session"
)

const (
	traceparentA = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	traceparentB = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
)

func TestTraceWriteDoesNotRaceWithStructCopy(t *testing.T) {
	root := session.NewContextWithSession(context.Background(), session.Session{
		Uri:    "/v1/orders",
		Method: "POST",
	})

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for range 10000 {
			ctx := session.WithTraceparent(root, traceparentA)
			ctx = session.WithTraceparent(ctx, traceparentB)
			_ = session.TraceparentFromContext(ctx)
		}
	}()

	go func() {
		defer wg.Done()
		for range 10000 {
			s := session.FromContext(root)
			copied := *s
			if copied.Uri != "/v1/orders" {
				t.Errorf("torn copy: Uri = %q", copied.Uri)
				return
			}
			_ = session.TraceparentFromContext(root)
		}
	}()

	wg.Wait()
}

func TestConcurrentDerivationFromOneParent(t *testing.T) {
	root := session.NewContextWithSession(context.Background(), session.Session{})

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 5000 {
				ctx := session.WithTrace(root, traceparentA, "vendor=a")
				if got := session.TraceparentFromContext(ctx); got != traceparentA {
					t.Errorf("Traceparent = %q, want %q", got, traceparentA)
					return
				}
			}
		})
	}
	wg.Wait()
}

func TestTraceDoesNotLeakIntoParentContext(t *testing.T) {
	request := session.WithTraceparent(
		session.NewContextWithSession(context.Background(), session.Session{}),
		traceparentA,
	)

	publish := func(ctx context.Context) {
		ctx = session.WithTraceparent(ctx, traceparentB)
		if got := session.TraceparentFromContext(ctx); got != traceparentB {
			t.Errorf("inside span: Traceparent = %q, want %q", got, traceparentB)
		}
	}
	publish(request)

	if got := session.TraceparentFromContext(request); got != traceparentA {
		t.Errorf("after span: Traceparent = %q, want %q", got, traceparentA)
	}
}

func TestWithTraceparentKeepsTracestate(t *testing.T) {
	ctx := session.WithTrace(context.Background(), traceparentA, "vendor=a")
	ctx = session.WithTraceparent(ctx, traceparentB)

	traceparent, tracestate := session.TraceFromContext(ctx)
	if traceparent != traceparentB {
		t.Errorf("traceparent = %q, want %q", traceparent, traceparentB)
	}
	if tracestate != "vendor=a" {
		t.Errorf("tracestate = %q, want %q", tracestate, "vendor=a")
	}
}

func TestWithTracestateKeepsTraceparent(t *testing.T) {
	ctx := session.WithTrace(context.Background(), traceparentA, "vendor=a")
	ctx = session.WithTracestate(ctx, "vendor=b")

	traceparent, tracestate := session.TraceFromContext(ctx)
	if traceparent != traceparentA {
		t.Errorf("traceparent = %q, want %q", traceparent, traceparentA)
	}
	if tracestate != "vendor=b" {
		t.Errorf("tracestate = %q, want %q", tracestate, "vendor=b")
	}
}

func TestFieldWrittenBeforeHandlerIsReadableAfter(t *testing.T) {
	ctx := session.NewContextWithSession(context.Background(), session.Session{
		CorrelationId: "corr-1",
		Uri:           "/v1/orders",
	})

	session.WithTraceparent(ctx, traceparentA)

	s := session.FromContext(ctx)
	if s.CorrelationId != "corr-1" {
		t.Errorf("CorrelationId = %q, want %q", s.CorrelationId, "corr-1")
	}
	if s.Uri != "/v1/orders" {
		t.Errorf("Uri = %q, want %q", s.Uri, "/v1/orders")
	}
}

func TestSetupGeneratesCorrelationId(t *testing.T) {
	if s := session.Setup(session.Session{}); s.CorrelationId == "" {
		t.Fatal("CorrelationId not generated")
	}
	if got := session.Setup(session.Session{CorrelationId: "given"}); got.CorrelationId != "given" {
		t.Errorf("CorrelationId = %q, want %q", got.CorrelationId, "given")
	}
}

func TestNewEmptyContextWithSessionCarriesTrace(t *testing.T) {
	ctx := session.NewContextWithSession(context.Background(), session.Session{CorrelationId: "corr-1"})
	ctx = session.WithTrace(ctx, traceparentA, "vendor=a")

	detached := session.NewEmptyContextWithSession(ctx)

	traceparent, tracestate := session.TraceFromContext(detached)
	if traceparent != traceparentA {
		t.Errorf("traceparent = %q, want %q", traceparent, traceparentA)
	}
	if tracestate != "vendor=a" {
		t.Errorf("tracestate = %q, want %q", tracestate, "vendor=a")
	}
	if s := session.FromContext(detached); s.CorrelationId != "corr-1" {
		t.Errorf("CorrelationId = %q, want %q", s.CorrelationId, "corr-1")
	}
	if detached.Done() != nil {
		t.Error("detached context inherited cancellation")
	}
}

func TestNewEmptyContextWithSessionWithoutSession(t *testing.T) {
	detached := session.NewEmptyContextWithSession(context.Background())
	s := session.FromContext(detached)
	if s == nil {
		t.Fatal("no session in detached context")
	}
	if s.CorrelationId == "" {
		t.Error("CorrelationId not generated")
	}
}

// Fails when a field is added: put it here only if nothing writes it after the
// session reaches a context, otherwise it belongs beside the trace.
func TestSessionCarriesOnlyWriteOnceFields(t *testing.T) {
	want := []string{
		"Uri", "Method", "Body", "CorrelationId",
		"UserAgent", "UserLanguage", "SourceIp",
	}

	typ := reflect.TypeFor[session.Session]()
	got := make([]string, 0, typ.NumField())
	for field := range typ.Fields() {
		got = append(got, field.Name)
	}

	if !slices.Equal(got, want) {
		t.Errorf("fields = %v, want %v", got, want)
	}
}

func TestFromContextWithoutSession(t *testing.T) {
	if s := session.FromContext(context.Background()); s != nil {
		t.Errorf("FromContext() = %v, want nil", s)
	}
	//nolint:staticcheck // SA1012: FromContext guards nil explicitly.
	if s := session.FromContext(nil); s != nil {
		t.Errorf("FromContext(nil) = %v, want nil", s)
	}
}

func TestTraceFromContextWithoutTrace(t *testing.T) {
	traceparent, tracestate := session.TraceFromContext(context.Background())
	if traceparent != "" || tracestate != "" {
		t.Errorf("TraceFromContext() = %q, %q, want empty", traceparent, tracestate)
	}
	//nolint:staticcheck // SA1012: TraceFromContext guards nil explicitly.
	if traceparent, tracestate = session.TraceFromContext(nil); traceparent != "" || tracestate != "" {
		t.Errorf("TraceFromContext(nil) = %q, %q, want empty", traceparent, tracestate)
	}
}
