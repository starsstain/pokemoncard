// Package observability emits Datadog-compatible structured logs.
package observability

import (
	"context"
	"encoding/json"
	"errors"
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

type requestErrorKey struct{}

// RequestError describes a failed HTTP request in terms Datadog recognizes.
// Keep this on the request context until the request-completed log is emitted.
type RequestError struct {
	Kind string
	Err  error
}

func (e *RequestError) Error() string { return e.Err.Error() }
func (e *RequestError) Unwrap() error { return e.Err }

// NewRequestError attaches a stable, searchable kind to an error.
func NewRequestError(kind string, err error) error {
	return &RequestError{Kind: kind, Err: err}
}

// WithRequestError records an error on a request context for terminal logging.
func WithRequestError(ctx context.Context, err error) context.Context {
	return context.WithValue(ctx, requestErrorKey{}, err)
}

type requestErrorState struct{ err error }

// NewRequestErrorContext creates request-local storage for terminal HTTP logs.
func NewRequestErrorContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, requestErrorKey{}, &requestErrorState{})
}

// RecordRequestError records the request's most relevant error in its context.
// NewRequestErrorContext must be called by the outer HTTP middleware first.
func RecordRequestError(ctx context.Context, kind string, err error) {
	if err == nil {
		return
	}
	state, _ := ctx.Value(requestErrorKey{}).(*requestErrorState)
	if state == nil {
		return
	}
	state.err = NewRequestError(kind, err)
}

// RequestErrorFields returns Datadog's standard error attributes, if present.
func RequestErrorFields(ctx context.Context) Fields {
	if ctx == nil {
		return nil
	}
	var err error
	switch value := ctx.Value(requestErrorKey{}).(type) {
	case *requestErrorState:
		err = value.err
	case error:
		err = value
	}
	if err == nil {
		return nil
	}
	fields := Fields{"error.message": err.Error()}
	var requestErr *RequestError
	if errors.As(err, &requestErr) && requestErr.Kind != "" {
		fields["error.kind"] = requestErr.Kind
	}
	return fields
}

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
		for key, value := range RequestErrorFields(ctx) {
			event[key] = value
		}
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
