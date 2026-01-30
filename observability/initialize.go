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

	"github.com/volcengine/veadk-go/configs"
	"github.com/volcengine/veadk-go/log"
	"github.com/volcengine/veadk-go/observability/exporter"
	"google.golang.org/adk/telemetry"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// AddSpanProcessor is a wrapper of google adk's RegisterSpanProcessor.
func AddSpanProcessor(processor sdktrace.SpanProcessor) {
	telemetry.RegisterSpanProcessor(processor)
}

// AddSpanExporter initializes the observability system by registering the exporter to
// Google ADK's local telemetry. It does NOT overwrite the global OTel TracerProvider.
func AddSpanExporter(exp sdktrace.SpanExporter) {
	// Always wrap with ADKTranslatedExporter to ensure ADK-internal spans are correctly mapped
	translatedExp := &exporter.ADKTranslatedExporter{SpanExporter: exp}
	AddSpanProcessor(sdktrace.NewBatchSpanProcessor(translatedExp))
}

// Init initializes the observability system using the global configuration.
// It automatically maps environment variables and YAML values.
func Init(ctx context.Context) error {
	globalConfig := configs.GetGlobalConfig()

	if globalConfig == nil || globalConfig.Observability == nil || globalConfig.Observability.OpenTelemetry == nil {
		log.Info("No observability config found, observability data will not be exported")
		return InitializeWithConfig(ctx, nil)
	}

	return InitializeWithConfig(ctx, globalConfig.Observability.OpenTelemetry)
}

// SetGlobalTracerProvider configures the global OpenTelemetry TracerProvider with the provided exporter.
// This is optional and used when you want unrelated OTel measurements to also be exported.
func SetGlobalTracerProvider(exp sdktrace.SpanExporter, enableMetrics bool, spanProcessors ...sdktrace.SpanProcessor) {
	// Always wrap with ADKTranslatedExporter to ensure ADK-internal spans are correctly mapped
	translatedExp := &exporter.ADKTranslatedExporter{SpanExporter: exp}

	// 1. Try to register with existing TracerProvider if it's an SDK TracerProvider
	globalTP := otel.GetTracerProvider()
	if sdkTP, ok := globalTP.(*sdktrace.TracerProvider); ok {
		log.Info("Registering ADK Exporter to existing global TracerProvider")
		for _, sp := range spanProcessors {
			sdkTP.RegisterSpanProcessor(sp)
		}
		sdkTP.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(translatedExp))
		return
	}

	// 2. Fallback: Overwrite with new TracerProvider
	log.Info("Creating a new global TracerProvider")
	var opts []sdktrace.TracerProviderOption
	for _, sp := range spanProcessors {
		opts = append(opts, sdktrace.WithSpanProcessor(sp))
	}

	tp := sdktrace.NewTracerProvider(
		append(opts, sdktrace.WithBatcher(translatedExp))...,
	)
	otel.SetTracerProvider(tp)
}

func setupLocalTracer(ctx context.Context, cfg *configs.OpenTelemetryConfig) error {

	if cfg == nil {
		return nil
	}

	exp, err := exporter.NewMultiExporter(ctx, cfg)
	if err != nil {
		return err
	}
	if exp != nil {
		AddSpanExporter(exp)
	}
	return nil
}

func setupGlobalTracer(ctx context.Context, cfg *configs.OpenTelemetryConfig) error {
	log.Info("Registering ADK Global TracerProvider")

	globalExp, err := exporter.NewMultiExporter(ctx, cfg)
	if err != nil {
		return err
	}
	if globalExp != nil {
		enableMetrics := cfg != nil && (cfg.EnableMetrics == nil || *cfg.EnableMetrics)
		SetGlobalTracerProvider(globalExp, enableMetrics)
	}
	return nil
}

func initializeTraceProvider(ctx context.Context, cfg *configs.OpenTelemetryConfig) error {
	var errs []error
	if cfg != nil && cfg.EnableLocalProvider {
		err := setupLocalTracer(ctx, cfg)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if cfg != nil && cfg.EnableGlobalProvider {
		err := setupGlobalTracer(ctx, cfg)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func initializeMeterProvider(ctx context.Context, cfg *configs.OpenTelemetryConfig) error {
	var errs []error
	if cfg == nil || cfg.EnableMetrics == nil || !*cfg.EnableMetrics {
		log.Info("Meter provider is not enabled")
		return nil
	}

	if cfg != nil && cfg.EnableLocalProvider {
		readers, err := exporter.NewMetricReader(ctx, cfg)
		if err != nil {
			errs = append(errs, err)
		}
		RegisterLocalMetrics(readers)
	}

	if cfg.EnableGlobalProvider {
		globalReaders, err := exporter.NewMetricReader(ctx, cfg)
		if err != nil {
			errs = append(errs, err)
		}
		RegisterGlobalMetrics(globalReaders)
	}
	return errors.Join(errs...)
}

// InitializeWithConfig automatically initializes the observability system based on the provided configuration.
// It creates the appropriate exporter and calls RegisterExporter.
func InitializeWithConfig(ctx context.Context, cfg *configs.OpenTelemetryConfig) error {
	var errs []error
	err := initializeTraceProvider(ctx, cfg)
	if err != nil {
		errs = append(errs, err)
	}

	err = initializeMeterProvider(ctx, cfg)
	if err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// Shutdown shuts down the observability system, flushing all spans and metrics.
func Shutdown(ctx context.Context) error {
	var errs []error

	// 1. Shutdown TracerProvider
	tp := otel.GetTracerProvider()
	if sdkTP, ok := tp.(*sdktrace.TracerProvider); ok {
		if err := sdkTP.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	// 2. Shutdown local MeterProvider if exists
	if localMeterProvider != nil {
		if err := localMeterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	// 3. Shutdown global MeterProvider if exists
	if globalMeterProvider != nil {
		if err := globalMeterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
