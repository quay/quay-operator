package tracing

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestSetupDisabledWithoutEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")

	shutdown, err := Setup(context.Background())
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		t.Fatal("global tracer provider is an SDK provider, want no-op")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestSetupEnabledWithEndpoint(t *testing.T) {
	for _, env := range []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
			t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
			t.Setenv(env, "http://127.0.0.1:4318")
			t.Cleanup(func() { otel.SetTracerProvider(noop.NewTracerProvider()) })

			shutdown, err := Setup(context.Background())
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}
			if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
				t.Fatalf("global tracer provider is %T, want SDK provider", otel.GetTracerProvider())
			}
			if err := shutdown(context.Background()); err != nil {
				t.Fatalf("shutdown: %v", err)
			}
		})
	}
}

func TestEndReconcile(t *testing.T) {
	for _, tc := range []struct {
		name    string
		outcome string
		res     ctrl.Result
		err     error
		want    string
		wantMs  int64
	}{
		{name: "error", err: errors.New("boom"), want: "error"},
		{name: "requeue", res: ctrl.Result{Requeue: true}, want: "requeue"},
		{name: "requeue after", res: ctrl.Result{RequeueAfter: 1500 * time.Millisecond}, want: "requeue", wantMs: 1500},
		{name: "success", want: "success"},
		{name: "override", outcome: "rollout_blocked", res: ctrl.Result{RequeueAfter: time.Second}, want: "rollout_blocked", wantMs: 1000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
			_, span := tp.Tracer("test").Start(context.Background(), "reconcile")

			EndReconcile(span, tc.outcome, tc.res, tc.err)

			spans := exporter.GetSpans()
			if len(spans) != 1 {
				t.Fatalf("got %d spans, want 1", len(spans))
			}
			attrs := attribute.NewSet(spans[0].Attributes...)
			if got, _ := attrs.Value("outcome"); got.AsString() != tc.want {
				t.Errorf("outcome = %q, want %q", got.AsString(), tc.want)
			}
			if got, _ := attrs.Value("requeue_after_ms"); got.AsInt64() != tc.wantMs {
				t.Errorf("requeue_after_ms = %d, want %d", got.AsInt64(), tc.wantMs)
			}
			if wantErr := tc.err != nil; (spans[0].Status.Code == codes.Error) != wantErr {
				t.Errorf("status = %v, want error %v", spans[0].Status.Code, wantErr)
			}
		})
	}
}
