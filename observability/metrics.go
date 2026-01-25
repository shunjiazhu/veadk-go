// Copyright (c) 2025 Beijing Volcano Engine Technology Co., Ltd. and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter = otel.Meter(InstrumentationName)

	// Metrics
	tokenUsageCounter, _          = meter.Int64Counter(MetricNameTokenUsage, metric.WithDescription("The number of tokens used in GenAI operations"))
	operationDurationHistogram, _ = meter.Float64Histogram(MetricNameOperationDuration, metric.WithDescription("GenAI operation duration in seconds"), metric.WithUnit("s"))
	firstTokenLatencyHistogram, _ = meter.Float64Histogram(MetricNameFirstTokenLatency, metric.WithDescription("Latency to the first token in seconds"), metric.WithUnit("s"))
)

// RecordTokenUsage records the number of tokens used.
func RecordTokenUsage(ctx context.Context, input, output int64, attrs ...attribute.KeyValue) {
	if input > 0 {
		tokenUsageCounter.Add(ctx, input, metric.WithAttributes(append(attrs, attribute.String("token.direction", "input"))...))
	}
	if output > 0 {
		tokenUsageCounter.Add(ctx, output, metric.WithAttributes(append(attrs, attribute.String("token.direction", "output"))...))
	}
}

// RecordOperationDuration records the duration of an operation.
func RecordOperationDuration(ctx context.Context, durationSeconds float64, attrs ...attribute.KeyValue) {
	operationDurationHistogram.Record(ctx, durationSeconds, metric.WithAttributes(attrs...))
}

// RecordFirstTokenLatency records the latency to the first token.
func RecordFirstTokenLatency(ctx context.Context, latencySeconds float64, attrs ...attribute.KeyValue) {
	firstTokenLatencyHistogram.Record(ctx, latencySeconds, metric.WithAttributes(attrs...))
}
