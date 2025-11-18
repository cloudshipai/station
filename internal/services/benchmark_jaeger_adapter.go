package services

import (
	"station/pkg/benchmark"
)

// BenchmarkJaegerAdapter adapts services.JaegerClient to benchmark.JaegerClientInterface
type BenchmarkJaegerAdapter struct {
	client *JaegerClient
}

// NewBenchmarkJaegerAdapter creates an adapter from services.JaegerClient
func NewBenchmarkJaegerAdapter(client *JaegerClient) *BenchmarkJaegerAdapter {
	return &BenchmarkJaegerAdapter{client: client}
}

// QueryRunTrace adapts the call and converts types
func (a *BenchmarkJaegerAdapter) QueryRunTrace(runID int64, serviceName string) (*benchmark.JaegerTrace, error) {
	// Call the underlying client
	svcTrace, err := a.client.QueryRunTrace(runID, serviceName)
	if err != nil {
		return nil, err
	}

	// Convert services.JaegerTrace to benchmark.JaegerTrace
	return convertTraceToBenchmark(svcTrace), nil
}

// IsAvailable checks if Jaeger is accessible
func (a *BenchmarkJaegerAdapter) IsAvailable() bool {
	return a.client.IsAvailable()
}

// convertTraceToBenchmark converts services.JaegerTrace to benchmark.JaegerTrace
func convertTraceToBenchmark(svcTrace *JaegerTrace) *benchmark.JaegerTrace {
	if svcTrace == nil {
		return nil
	}

	trace := &benchmark.JaegerTrace{
		TraceID:   svcTrace.TraceID,
		Spans:     make([]benchmark.JaegerSpan, len(svcTrace.Spans)),
		Processes: make(map[string]benchmark.JaegerProcess),
	}

	// Convert spans
	for i, svcSpan := range svcTrace.Spans {
		trace.Spans[i] = benchmark.JaegerSpan{
			TraceID:       svcSpan.TraceID,
			SpanID:        svcSpan.SpanID,
			OperationName: svcSpan.OperationName,
			References:    convertReferencesToBenchmark(svcSpan.References),
			StartTime:     svcSpan.StartTime,
			Duration:      svcSpan.Duration,
			Tags:          convertTagsToBenchmark(svcSpan.Tags),
			Logs:          convertLogsToBenchmark(svcSpan.Logs),
			ProcessID:     svcSpan.ProcessID,
		}
	}

	// Convert processes
	for key, svcProc := range svcTrace.Processes {
		trace.Processes[key] = benchmark.JaegerProcess{
			ServiceName: svcProc.ServiceName,
			Tags:        convertTagsToBenchmark(svcProc.Tags),
		}
	}

	return trace
}

func convertReferencesToBenchmark(svcRefs []SpanRef) []benchmark.SpanRef {
	refs := make([]benchmark.SpanRef, len(svcRefs))
	for i, svcRef := range svcRefs {
		refs[i] = benchmark.SpanRef{
			RefType: svcRef.RefType,
			TraceID: svcRef.TraceID,
			SpanID:  svcRef.SpanID,
		}
	}
	return refs
}

func convertTagsToBenchmark(svcTags []SpanTag) []benchmark.SpanTag {
	tags := make([]benchmark.SpanTag, len(svcTags))
	for i, svcTag := range svcTags {
		tags[i] = benchmark.SpanTag{
			Key:   svcTag.Key,
			Type:  svcTag.Type,
			Value: svcTag.Value,
		}
	}
	return tags
}

func convertLogsToBenchmark(svcLogs []SpanLog) []benchmark.SpanLog {
	logs := make([]benchmark.SpanLog, len(svcLogs))
	for i, svcLog := range svcLogs {
		logs[i] = benchmark.SpanLog{
			Timestamp: svcLog.Timestamp,
			Fields:    convertTagsToBenchmark(svcLog.Fields),
		}
	}
	return logs
}
