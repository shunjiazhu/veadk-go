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

	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

var (
	// Slices to hold instruments from multiple providers (Global, Local, etc.)
	instrumentsMu               sync.RWMutex
	tokenUsageCounters          []metric.Int64Counter
	operationDurationHistograms []metric.Float64Histogram
	firstTokenLatencyHistograms []metric.Float64Histogram

	localOnce  sync.Once
	globalOnce sync.Once
)

func registerMeter(meter metric.Meter) {
	instrumentsMu.Lock()
	defer instrumentsMu.Unlock()

	if c, err := meter.Int64Counter(MetricNameTokenUsage, metric.WithDescription("The number of tokens used in GenAI operations")); err == nil {
		tokenUsageCounters = append(tokenUsageCounters, c)
	}
	if h, err := meter.Float64Histogram(MetricNameOperationDuration, metric.WithDescription("GenAI operation duration in seconds"), metric.WithUnit("s")); err == nil {
		operationDurationHistograms = append(operationDurationHistograms, h)
	}
	if h, err := meter.Float64Histogram(MetricNameFirstTokenLatency, metric.WithDescription("Latency to the first token in seconds"), metric.WithUnit("s")); err == nil {
		firstTokenLatencyHistograms = append(firstTokenLatencyHistograms, h)
	}
}

// RegisterMetrics initializes the metrics system with a local isolated MeterProvider.
// It does NOT overwrite the global OTel MeterProvider.
func RegisterMetrics(readers []sdkmetric.Reader, serviceName string) {
	if len(readers) == 0 {
		return
	}
	localOnce.Do(func() {
		res, _ := resource.Merge(
			resource.Default(),
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(serviceName),
			),
		)

		options := []sdkmetric.Option{
			sdkmetric.WithResource(res),
		}
		for _, r := range readers {
			options = append(options, sdkmetric.WithReader(r))
		}

		mp := sdkmetric.NewMeterProvider(options...)
		registerMeter(mp.Meter(InstrumentationName))
	})
}

// RegisterGlobalMetrics configures the global OpenTelemetry MeterProvider with the provided readers.
// This is optional and used when you want unrelated OTel measurements to also be exported.
func RegisterGlobalMetrics(readers []sdkmetric.Reader, serviceName string) {
	if len(readers) == 0 {
		return
	}
	globalOnce.Do(func() {
		res, _ := resource.Merge(
			resource.Default(),
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String(serviceName),
			),
		)

		options := []sdkmetric.Option{
			sdkmetric.WithResource(res),
		}
		for _, r := range readers {
			options = append(options, sdkmetric.WithReader(r))
		}

		mp := sdkmetric.NewMeterProvider(options...)
		otel.SetMeterProvider(mp)
		// No need to call registerMeter here, because the global proxy registered in init()
		registerMeter(otel.GetMeterProvider().Meter(InstrumentationName))
	})
}

// RecordTokenUsage records the number of tokens used.
func RecordTokenUsage(ctx context.Context, input, output int64, attrs ...attribute.KeyValue) {
	instrumentsMu.RLock()
	defer instrumentsMu.RUnlock()

	for _, counter := range tokenUsageCounters {
		if input > 0 {
			counter.Add(ctx, input, metric.WithAttributes(append(attrs, attribute.String("token.direction", "input"))...))
		}
		if output > 0 {
			counter.Add(ctx, output, metric.WithAttributes(append(attrs, attribute.String("token.direction", "output"))...))
		}
	}
}

// RecordOperationDuration records the duration of an operation.
func RecordOperationDuration(ctx context.Context, durationSeconds float64, attrs ...attribute.KeyValue) {
	instrumentsMu.RLock()
	defer instrumentsMu.RUnlock()

	for _, histogram := range operationDurationHistograms {
		histogram.Record(ctx, durationSeconds, metric.WithAttributes(attrs...))
	}
}

// RecordFirstTokenLatency records the latency to the first token.
func RecordFirstTokenLatency(ctx context.Context, latencySeconds float64, attrs ...attribute.KeyValue) {
	instrumentsMu.RLock()
	defer instrumentsMu.RUnlock()

	for _, histogram := range firstTokenLatencyHistograms {
		histogram.Record(ctx, latencySeconds, metric.WithAttributes(attrs...))
	}
}
