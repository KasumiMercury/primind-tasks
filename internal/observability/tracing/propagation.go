package tracing

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func ExtractFromHTTPRequest(ctx context.Context, r *http.Request) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))
}

func InjectToHTTPRequest(ctx context.Context, r *http.Request) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(r.Header))
}

// for message attributes
func ExtractFromMap(ctx context.Context, carrier map[string]string) context.Context {
	if ctxWith, ok := extractFromGCPAttributes(ctx, carrier); ok {
		return ctxWith
	}

	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(carrier))
}

// for message attributes
func InjectToMap(ctx context.Context, carrier map[string]string) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(carrier))
}

func extractFromGCPAttributes(ctx context.Context, carrier map[string]string) (context.Context, bool) {
	traceIDValue := carrier["googclient_trace_id"]
	spanIDValue := carrier["googclient_span_id"]
	if traceIDValue == "" || spanIDValue == "" {
		return ctx, false
	}

	traceID, ok := parseTraceID(traceIDValue)
	if !ok {
		return ctx, false
	}

	spanID, ok := parseSpanID(spanIDValue)
	if !ok {
		return ctx, false
	}

	flags := parseTraceFlags(carrier["googclient_sampling"], carrier["googclient_sampled"])
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: flags,
		TraceState: trace.TraceState{},
		Remote:     true,
	})
	if !spanContext.IsValid() {
		return ctx, false
	}

	return trace.ContextWithRemoteSpanContext(ctx, spanContext), true
}

func parseTraceID(value string) (trace.TraceID, bool) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "0x"))
	traceID, err := trace.TraceIDFromHex(trimmed)
	if err != nil {
		return trace.TraceID{}, false
	}

	return traceID, true
}

func parseSpanID(value string) (trace.SpanID, bool) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(value, "0x"))
	spanID, err := trace.SpanIDFromHex(trimmed)
	if err == nil {
		return spanID, true
	}

	if trimmed == "" {
		return trace.SpanID{}, false
	}

	parsed, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil {
		return trace.SpanID{}, false
	}

	spanID, err = trace.SpanIDFromHex(fmt.Sprintf("%016x", parsed))
	if err != nil {
		return trace.SpanID{}, false
	}

	return spanID, true
}

func parseTraceFlags(values ...string) trace.TraceFlags {
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		switch normalized {
		case "1", "true", "sampled":
			return trace.FlagsSampled
		}
	}

	return 0
}
