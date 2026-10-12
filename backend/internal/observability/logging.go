// Package observability emits Datadog-compatible structured logs.
package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

const (
	Environment = "school"
	Version     = "1.0.0"
	Team        = "bj-xsolla-school-group-2"
)

// Fields are additional JSON attributes for a log event.
type Fields map[string]any

// Log writes a single JSON record. Datadog automatically recognizes the
// standard fields and correlates dd.trace_id/dd.span_id with APM traces.
func Log(ctx context.Context, service, status, message string, fields Fields) {
	event := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"service":   service,
		"env":       Environment,
		"version":   Version,
		"team":      Team,
		"status":    status,
		"message":   message,
	}
	for key, value := range fields {
		event[key] = value
	}
	if ctx != nil {
		if span, ok := tracer.SpanFromContext(ctx); ok {
			event["dd.trace_id"] = strconv.FormatUint(span.Context().TraceID(), 10)
			event["dd.span_id"] = strconv.FormatUint(span.Context().SpanID(), 10)
		}
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		encoded = []byte(fmt.Sprintf(`{"service":%q,"status":"error","message":"could not encode structured log"}`, service))
	}
	fmt.Fprintln(os.Stdout, string(encoded))
}

// StatusForHTTP maps failed HTTP responses to searchable Datadog severities.
func StatusForHTTP(statusCode int) string {
	switch {
	case statusCode >= 500:
		return "error"
	case statusCode >= 400:
		return "warn"
	default:
		return "info"
	}
}
