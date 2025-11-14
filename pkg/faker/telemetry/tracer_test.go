package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

func TestNewTracer_NoEndpoint(t *testing.T) {
	// Should return no-op tracer when endpoint is empty
	tracer, err := NewTracer("", false)
	require.NoError(t, err)
	assert.NotNil(t, tracer)

	// Verify it's the no-op implementation
	_, ok := tracer.(*noopTracer)
	assert.True(t, ok, "should return noopTracer when endpoint is empty")
}

func TestNewTracer_WithEndpoint(t *testing.T) {
	// Note: This will fail to connect but should create the tracer
	tracer, err := NewTracer("localhost:4318", false)

	// Should create tracer even if endpoint is unreachable
	require.NoError(t, err)
	assert.NotNil(t, tracer)
}

func TestTracer_StartSpan(t *testing.T) {
	tracer, err := NewTracer("", false) // no-op tracer
	require.NoError(t, err)

	ctx := context.Background()
	ctx, span := tracer.StartSpan(ctx, "test-span",
		attribute.String("test.key", "test.value"))

	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	span.End()
}

func TestTracer_RecordError(t *testing.T) {
	tracer, err := NewTracer("", false)
	require.NoError(t, err)

	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "test-span")

	testErr := assert.AnError
	tracer.RecordError(span, testErr)

	span.End()
}

func TestTracer_RecordEvent(t *testing.T) {
	tracer, err := NewTracer("", false)
	require.NoError(t, err)

	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "test-span")

	tracer.RecordEvent(span, "test-event",
		attribute.String("event.key", "event.value"))

	span.End()
}

func TestTracer_SetStatus(t *testing.T) {
	tracer, err := NewTracer("", false)
	require.NoError(t, err)

	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "test-span")

	tracer.SetStatus(span, codes.Error, "test error")

	span.End()
}

func TestTracer_Shutdown(t *testing.T) {
	tracer, err := NewTracer("", false)
	require.NoError(t, err)

	ctx := context.Background()
	err = tracer.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestNoopTracer_AllMethods(t *testing.T) {
	tracer := &noopTracer{}
	ctx := context.Background()

	// All methods should work without error
	ctx, span := tracer.StartSpan(ctx, "test")
	assert.NotNil(t, ctx)
	assert.NotNil(t, span)

	tracer.RecordError(span, assert.AnError)
	tracer.RecordEvent(span, "event")
	tracer.SetStatus(span, codes.Error, "error")

	err := tracer.Shutdown(ctx)
	assert.NoError(t, err)
}
