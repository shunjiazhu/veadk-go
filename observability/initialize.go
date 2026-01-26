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
	"errors"
	"os"

	"github.com/volcengine/veadk-go/configs"
	"github.com/volcengine/veadk-go/log"
	"github.com/volcengine/veadk-go/observability/exporter"
	"google.golang.org/adk/telemetry"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// RegisterSpanProcessor is a wrapper of google adk's RegisterSpanProcessor.
func RegisterSpanProcessor(processor sdktrace.SpanProcessor) {
	telemetry.RegisterSpanProcessor(processor)
}

// RegisterSpanExporter initializes the observability system by registering the exporter to
// Google ADK's local telemetry. It does NOT overwrite the global OTel TracerProvider.
func RegisterSpanExporter(exp sdktrace.SpanExporter) {
	RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(&exporter.TranslatedExporter{SpanExporter: exp}))
}

// Init initializes the observability system using the global configuration.
// It automatically maps environment variables and YAML values.
func Init(ctx context.Context) error {
	globalConfig := configs.GetGlobalConfig()

	if globalConfig == nil || globalConfig.Observability == nil || globalConfig.Observability.OpenTelemetry == nil {
		log.Info("No observability config found, observability data will not be exported")
		return InitWithConfig(ctx, nil)
	}

	return InitWithConfig(ctx, globalConfig.Observability.OpenTelemetry)
}

// RegisterGlobalTracer configures the global OpenTelemetry TracerProvider with the provided exporter.
// This is optional and used when you want unrelated OTel measurements to also be exported.
func RegisterGlobalTracer(exporter sdktrace.SpanExporter, spanProcessors ...sdktrace.SpanProcessor) {
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithSpanProcessor(&VeSpanEnrichmentProcessor{}),
	}
	for _, sp := range spanProcessors {
		opts = append(opts, sdktrace.WithSpanProcessor(sp))
	}
	tp := sdktrace.NewTracerProvider(
		append(opts, sdktrace.WithBatcher(exporter))...,
	)
	otel.SetTracerProvider(tp)
	log.Info("Registered global TracerProvider with exporter")
}

func registerLocalTracer(ctx context.Context, cfg *configs.OpenTelemetryConfig) error {
	log.Info("Registered VeSpanEnrichmentProcessor for ADK Local TracerProvider")
	RegisterSpanProcessor(&VeSpanEnrichmentProcessor{})

	exp, err := exporter.NewMultiSpanExporter(ctx, cfg)
	if err != nil {
		return err
	}
	RegisterSpanExporter(exp)
	return nil
}

func registerGlobalTracer(ctx context.Context, cfg *configs.OpenTelemetryConfig) error {
	log.Info("Registered VeSpanEnrichmentProcessor for ADK Global TracerProvider")

	globalExp, err := exporter.NewMultiSpanExporter(ctx, cfg)
	if err != nil {
		return err
	}
	RegisterGlobalTracer(globalExp)
	return nil
}

func initTraceProvider(ctx context.Context, cfg *configs.OpenTelemetryConfig) error {
	var errs []error
	err := registerLocalTracer(ctx, cfg)
	if err != nil {
		errs = append(errs, err)
	}

	if cfg.EnableGlobalProvider {
		err = registerGlobalTracer(ctx, cfg)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func initMeterProvider(ctx context.Context, cfg *configs.OpenTelemetryConfig) error {
	readers, err := exporter.NewMetricReader(ctx, cfg)
	if err != nil {
		return err
	}
	RegisterLocalMetrics(readers)

	if cfg.EnableGlobalProvider {
		globalReaders, err := exporter.NewMetricReader(ctx, cfg)
		if err != nil {
			return err
		}
		RegisterGlobalMetrics(globalReaders)
	}
	return nil
}

// InitWithConfig automatically initializes the observability system based on the provided configuration.
// It creates the appropriate exporter and calls RegisterExporter.
func InitWithConfig(ctx context.Context, cfg *configs.OpenTelemetryConfig) error {
	var errs []error
	err := initTraceProvider(ctx, cfg)
	if err != nil {
		errs = append(errs, err)
	}

	err = initMeterProvider(ctx, cfg)
	if err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func getServiceName(cfg *configs.OpenTelemetryConfig) string {
	if serviceFromEnv := os.Getenv("OTEL_SERVICE_NAME"); serviceFromEnv != "" {
		return serviceFromEnv
	}

	if cfg.ApmPlus != nil {
		if cfg.ApmPlus.ServiceName != "" {
			return cfg.ApmPlus.ServiceName
		}
	}

	if cfg.CozeLoop != nil {
		if cfg.CozeLoop.ServiceName != "" {
			return cfg.CozeLoop.ServiceName
		}
	}

	if cfg.TLS != nil {
		if cfg.TLS.ServiceName != "" {
			return cfg.TLS.ServiceName
		}
	}
	return "<unknown_service>"
}
