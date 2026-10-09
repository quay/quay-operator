// Package tracing configures opt-in OpenTelemetry tracing for the operator's reconcilers.
package tracing

import (
	"context"
	"net/http"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

const tracerName = "github.com/quay/quay-operator"

// Span attribute keys. Trace queries depend on these names.
const (
	keyRegistryNamespace = attribute.Key("quay.registry.namespace")
	keyRegistryName      = attribute.Key("quay.registry.name")
	keyRegistryUID       = attribute.Key("quay.registry.uid")
	keyRegistryGen       = attribute.Key("quay.registry.generation")
	keyReconcileID       = attribute.Key("quay.reconcile.id")
	keyOutcome           = attribute.Key("quay.reconcile.outcome")
	keyWaitReason        = attribute.Key("quay.reconcile.wait_reason")
	keyRequeueAfterMs    = attribute.Key("quay.reconcile.requeue_after_ms")

	KeyConditionReason = attribute.Key("quay.reconcile.condition_reason")
	KeyDeferWorkloads  = attribute.Key("quay.apply.defer_workloads")
	KeyComponent       = attribute.Key("quay.component")
)

// Setup installs a global OTLP/HTTP tracer provider when an OTLP endpoint is configured
// through the standard OTEL_* environment variables. Without one the global no-op provider
// is left in place and cfg is not modified. Otherwise cfg's transport is wrapped so
// Kubernetes API requests are traced. The returned function flushes and stops the provider.
func Setup(ctx context.Context, cfg *rest.Config) (func(context.Context) error, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" &&
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}

	// WithFromEnv comes last so OTEL_SERVICE_NAME overrides the default service name.
	res, err := resource.New(
		ctx,
		resource.WithAttributes(semconv.ServiceName("quay-operator")),
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	cfg.Wrap(func(rt http.RoundTripper) http.RoundTripper {
		return otelhttp.NewTransport(rt)
	})
	return tp.Shutdown, nil
}

// Start starts a span using the operator's tracer.
func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, name, trace.WithAttributes(attrs...))
}

// StartReconcile starts the root span of a reconcile for the given request.
func StartReconcile(ctx context.Context, name string, req ctrl.Request) (context.Context, trace.Span) {
	return Start(
		ctx, name,
		keyRegistryNamespace.String(req.Namespace),
		keyRegistryName.String(req.Name),
		keyReconcileID.String(string(controller.ReconcileIDFromContext(ctx))),
	)
}

// SetRegistry records the identity of the reconciled object on the current span.
func SetRegistry(ctx context.Context, obj metav1.Object) {
	trace.SpanFromContext(ctx).SetAttributes(
		keyRegistryUID.String(string(obj.GetUID())),
		keyRegistryGen.Int64(obj.GetGeneration()),
	)
}

// SetWaitReason records why the current reconcile stops early or requeues.
func SetWaitReason(ctx context.Context, reason string, attrs ...attribute.KeyValue) {
	trace.SpanFromContext(ctx).SetAttributes(append(attrs, keyWaitReason.String(reason))...)
}

// RecordError marks the current span as failed.
func RecordError(ctx context.Context, err error) {
	fail(trace.SpanFromContext(ctx), err)
}

func fail(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// EndReconcile records the reconcile result and ends its root span. An empty outcome is
// derived from the result and error.
func EndReconcile(span trace.Span, outcome string, res ctrl.Result, err error) {
	if outcome == "" {
		switch {
		case err != nil:
			outcome = "error"
		case res.Requeue || res.RequeueAfter > 0:
			outcome = "requeue"
		default:
			outcome = "success"
		}
	}
	if err != nil {
		fail(span, err)
	}
	span.SetAttributes(
		keyOutcome.String(outcome),
		keyRequeueAfterMs.Int64(res.RequeueAfter.Milliseconds()),
	)
	span.End()
}
