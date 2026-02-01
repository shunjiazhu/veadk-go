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
	"os/signal"
	"syscall"

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

// AddSpanExporter registers an exporter to Google ADK's local telemetry.
func AddSpanExporter(exp sdktrace.SpanExporter) {
	// Always wrap with ADKTranslatedExporter to ensure ADK-internal spans are correctly mapped
	translatedExp := &exporter.ADKTranslatedExporter{SpanExporter: exp}
	AddSpanProcessor(exporter.NewADKSpanProcessor())

	// Use BatchSpanProcessor for better performance and batching.
	// Data durability is ensured via Shutdown/Flush on exit.
	AddSpanProcessor(sdktrace.NewBatchSpanProcessor(translatedExp))
}

// Init initializes the observability system using the global configuration.
// It automatically maps environment variables and YAML values.
func Init(ctx context.Context) error {
	HandleSignals(ctx)
	globalConfig := configs.GetGlobalConfig()

	if globalConfig == nil || globalConfig.Observability == nil || globalConfig.Observability.OpenTelemetry == nil {
		log.Info("No observability config found, observability data will not be exported")
		return InitializeWithConfig(ctx, nil)
	}

	return InitializeWithConfig(ctx, globalConfig.Observability.OpenTelemetry)
}

// SetGlobalTracerProvider configures the global OpenTelemetry TracerProvider.
func SetGlobalTracerProvider(exp sdktrace.SpanExporter, enableMetrics bool, spanProcessors ...sdktrace.SpanProcessor) {
	// Always wrap with ADKTranslatedExporter to ensure ADK-internal spans are correctly mapped
	translatedExp := &exporter.ADKTranslatedExporter{SpanExporter: exp}

	// Default processors
	allProcessors := append([]sdktrace.SpanProcessor{exporter.NewADKSpanProcessor()}, spanProcessors...)

	// Use BatchSpanProcessor for all exporters to ensure performance and batching.
	finalProcessor := sdktrace.NewBatchSpanProcessor(translatedExp)

	// 1. Try to register with existing TracerProvider if it's an SDK TracerProvider
	globalTP := otel.GetTracerProvider()
	if sdkTP, ok := globalTP.(*sdktrace.TracerProvider); ok {
		log.Info("Registering ADK Processors to existing global TracerProvider")
		for _, sp := range allProcessors {
			sdkTP.RegisterSpanProcessor(sp)
		}
		sdkTP.RegisterSpanProcessor(finalProcessor)
		return
	}

	// 2. Fallback: Create a new global TracerProvider
	log.Info("Creating a new global TracerProvider")
	var opts []sdktrace.TracerProviderOption
	for _, sp := range allProcessors {
		opts = append(opts, sdktrace.WithSpanProcessor(sp))
	}

	tp := sdktrace.NewTracerProvider(
		append(opts, sdktrace.WithSpanProcessor(finalProcessor))...,
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

	// 0. End all active root invocation spans to ensure they are recorded and flushed.
	// This handles cases like Ctrl+C or premature exit where defer blocks might not run.
	exporter.EndAllInvocationSpans()

	// 1. Shutdown TracerProvider
	tp := otel.GetTracerProvider()
	if sdkTP, ok := tp.(*sdktrace.TracerProvider); ok {
		log.Info("Shutting down TracerProvider and flushing spans")
		if err := sdkTP.ForceFlush(ctx); err != nil {
			log.Error("Failed to force flush TracerProvider", "err", err)
			errs = append(errs, err)
		}

		if err := sdkTP.Shutdown(ctx); err != nil {
			log.Error("Failed to shutdown TracerProvider", "err", err)
			errs = append(errs, err)
		}
	} else {
		log.Info("Global TracerProvider is not an SDK TracerProvider, skipping shutdown")
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

// HandleSignals registers a signal handler to ensure observability data is flushed on exit.
func HandleSignals(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info("Received signal, performing graceful shutdown", "signal", sig)

		// Trigger shutdown which will flush all processors (including BatchSpanProcessor)
		_ = Shutdown(ctx)
		os.Exit(0)
	}()
}
