// Package otel sets up OpenTelemetry tracing for Go services. It is the Go
// equivalent of company-observability-starter's micrometer + OTLP wiring.
//
// Usage:
//
//	shutdown, err := otel.Setup(ctx, otel.Config{ServiceName: "market-service-go"})
//	if err != nil { return err }
//	defer shutdown(context.Background())
//
// OTEL is always enabled unless OTEL_SDK_DISABLED=true or
// OTEL_EXPORTER_OTLP_ENDPOINT is empty. If the exporter cannot reach the
// collector at runtime, the SDK logs and keeps running — the service does
// not fail.
//
// HTTP and gRPC instrumentation helpers are exposed so each service wires
// them into its existing middleware/interceptor chain.
package otel

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"banka1/go-platform/config"
	"banka1/go-platform/httpx"
	"banka1/go-platform/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// Config selects exporter endpoint and toggle. Leave Endpoint blank to
// disable tracing entirely; the returned ShutdownFunc is then a no-op.
type Config struct {
	ServiceName string
	Endpoint    string // OTEL_EXPORTER_OTLP_ENDPOINT (HTTP base URL)
	Sampling    float64
	Disabled    bool
}

// LoadConfig reads OTEL_* env vars.
func LoadConfig(defaultServiceName string) Config {
	endpoint := config.Env("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	return Config{
		ServiceName: config.Env("OTEL_SERVICE_NAME", defaultServiceName),
		Endpoint:    endpoint,
		Sampling:    envFloat("OTEL_TRACES_SAMPLER_ARG", 1.0),
		Disabled:    config.EnvBool("OTEL_SDK_DISABLED", false) || endpoint == "",
	}
}

// ShutdownFunc flushes pending spans and shuts the exporter down.
type ShutdownFunc func(context.Context) error

// Setup configures the global TracerProvider and Propagator.
// Returns a ShutdownFunc the caller must call before exit (typically deferred
// from main).
func Setup(ctx context.Context, cfg Config) (ShutdownFunc, error) {
	if cfg.Disabled {
		return func(context.Context) error { return nil }, nil
	}
	endpoint, secure := normalizeOTLPEndpoint(cfg.Endpoint)
	options := []otlptracehttp.Option{otlptracehttp.WithEndpoint(endpoint)}
	if !secure {
		options = append(options, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("otel: exporter: %w", err)
	}
	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("otel: resource: %w", err)
	}
	sampling := cfg.Sampling
	if sampling <= 0 {
		sampling = 1.0
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(2*time.Second)),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampling))),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return func(shutdownCtx context.Context) error {
		return tp.Shutdown(shutdownCtx)
	}, nil
}

func normalizeOTLPEndpoint(raw string) (host string, secure bool) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "https://") {
		return strings.TrimPrefix(trimmed, "https://"), true
	}
	if strings.HasPrefix(trimmed, "http://") {
		return strings.TrimPrefix(trimmed, "http://"), false
	}
	return trimmed, false
}

func envFloat(key string, fallback float64) float64 {
	raw := strings.TrimSpace(config.Env(key, ""))
	if raw == "" {
		return fallback
	}
	var v float64
	if _, err := fmt.Sscanf(raw, "%f", &v); err != nil {
		return fallback
	}
	return v
}

// HTTPMiddleware wraps each request in a server span and enriches the
// context-bound logger with traceId/spanId so httpx.RequestLogMiddleware
// surfaces them automatically.
func HTTPMiddleware(serviceName string, baseLogger *slog.Logger) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)
	prop := otel.GetTextMapPropagator()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := prop.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			spanCtx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path)
			defer span.End()
			lg := baseLogger
			if sc := span.SpanContext(); sc.IsValid() {
				lg = baseLogger.With(
					slog.String("traceId", sc.TraceID().String()),
					slog.String("spanId", sc.SpanID().String()),
				)
			}
			spanCtx = log.WithLogger(spanCtx, lg)
			// Echo correlation id into the span so traces can be correlated with logs.
			if corr := httpx.CorrelationFromContext(spanCtx); corr != "" {
				span.SetAttributes(attribute.String("correlation.id", corr))
			}
			next.ServeHTTP(w, r.WithContext(spanCtx))
		})
	}
}

// UnaryServerInterceptor is the gRPC equivalent of HTTPMiddleware.
func UnaryServerInterceptor(serviceName string, baseLogger *slog.Logger) grpc.UnaryServerInterceptor {
	tracer := otel.Tracer(serviceName)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, span := tracer.Start(ctx, info.FullMethod)
		defer span.End()
		lg := baseLogger
		if sc := span.SpanContext(); sc.IsValid() {
			lg = baseLogger.With(
				slog.String("traceId", sc.TraceID().String()),
				slog.String("spanId", sc.SpanID().String()),
			)
		}
		ctx = log.WithLogger(ctx, lg)
		return handler(ctx, req)
	}
}

// TraceID returns the active trace id for ctx, or empty when no span.
func TraceID(ctx context.Context) string {
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		return span.TraceID().String()
	}
	return ""
}

// SpanID returns the active span id for ctx, or empty when no span.
func SpanID(ctx context.Context) string {
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		return span.SpanID().String()
	}
	return ""
}
