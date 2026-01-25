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
	"fmt"

	"github.com/volcengine/veadk-go/configs"
	"github.com/volcengine/veadk-go/observability/exporter"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/adk/telemetry"
)

// NewExporter creates a span exporter based on the provided configuration and wraps it with TranslatedExporter.
func NewExporter(ctx context.Context, cfg exporter.Config) (sdktrace.SpanExporter, error) {
	var exp sdktrace.SpanExporter
	var err error

	switch cfg.ExporterType {
	case exporter.ExporterCozeLoop:
		exp, err = exporter.NewCozeLoopExporter(ctx, cfg)
	case exporter.ExporterAPMPlus:
		exp, err = exporter.NewAPMPlusExporter(ctx, cfg)
	case exporter.ExporterTLS:
		exp, err = exporter.NewTLSExporter(ctx, cfg)
	case exporter.ExporterStdout:
		exp, err = exporter.NewStdoutExporter()
	case exporter.ExporterFile:
		exp, err = exporter.NewFileExporter(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", cfg.ExporterType)
	}

	if err != nil {
		return nil, err
	}

	return &TranslatedExporter{SpanExporter: exp}, nil
}

func init() {
	RegisterSpanProcessor(&VeSpanEnrichmentProcessor{})
}

func RegisterSpanProcessor(processor sdktrace.SpanProcessor) {
	telemetry.RegisterSpanProcessor(processor)
}

// RegisterExporter initializes the observability system by registering the exporter to
// Google ADK's local telemetry. It does NOT overwrite the global OTel TracerProvider.
func RegisterExporter(exp sdktrace.SpanExporter) {
	exporter := &TranslatedExporter{SpanExporter: exp}
	telemetry.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter))
}

// Init initializes the observability system using the global configuration.
// It automatically maps environment variables and YAML values.
func Init(ctx context.Context) error {
	cfg, ok := exporter.ToObservabilityConfig(configs.GetGlobalConfig().Observability)
	if !ok {
		// If no observability config is found, we don't return an error
		// as observability is often optional.
		return nil
	}
	return InitWithConfig(ctx, cfg)
}

// RegisterGlobalTracer configures the global OpenTelemetry TracerProvider with the provided exporter.
// This is optional and used when you want unrelated OTel measurements to also be exported.
func RegisterGlobalTracer(exporter sdktrace.SpanExporter, serviceName string) {
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(&VeSpanEnrichmentProcessor{}),
	)
	otel.SetTracerProvider(tp)
}

// InitWithConfig automatically initializes the observability system based on the provided configuration.
// It creates the appropriate exporter and calls RegisterExporter.
func InitWithConfig(ctx context.Context, cfg exporter.Config) error {
	exp, err := NewExporter(ctx, cfg)
	if err != nil {
		return err
	}

	RegisterExporter(exp)

	// Setup metrics if reader can be created
	if mr, err := exporter.NewMetricReader(ctx, cfg); err == nil {
		RegisterMetrics(mr, cfg.ServiceName)

		// Optionally setup global tracer/metrics if requested
		if cfg.EnableGlobalTracer {
			RegisterGlobalTracer(exp, cfg.ServiceName)
			RegisterGlobalMetrics(mr, cfg.ServiceName)
		}
	} else {
		// Fallback: If metrics setup failed (e.g. unsupported type), ensure at least Global Tracer is set if requested
		if cfg.EnableGlobalTracer {
			RegisterGlobalTracer(exp, cfg.ServiceName)
		}
	}

	return nil
}
