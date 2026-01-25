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

package exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// NewStdoutExporter creates a simple stdout exporter with pretty printing.
func NewStdoutExporter() (trace.SpanExporter, error) {
	return stdouttrace.New(stdouttrace.WithPrettyPrint())
}

// NewCozeLoopExporter creates an OTLP HTTP exporter for CozeLoop.
func NewCozeLoopExporter(ctx context.Context, cfg *CozeLoopConfig) (trace.SpanExporter, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "api.coze.cn" // Default Coze domain
	}

	options := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": "Bearer " + cfg.APIKey,
			"X-Coze-Space":  cfg.ServiceName,
		}),
	}

	if !strings.HasPrefix(endpoint, "http") {
		// If no scheme, assume HTTPS for Coze
		options = append(options, otlptracehttp.WithInsecure())
	}

	return otlptrace.New(ctx, otlptracehttp.NewClient(options...))
}

// NewAPMPlusExporter creates an OTLP HTTP exporter for APMPlus.
func NewAPMPlusExporter(ctx context.Context, cfg *ApmPlusConfig) (trace.SpanExporter, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "apmplus-cn-beijing.volces.com:4317"
	}

	options := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": cfg.APIKey,
		}),
	}

	options = append(options, otlptracehttp.WithInsecure())

	return otlptrace.New(ctx, otlptracehttp.NewClient(options...))
}

// NewTLSExporter creates an OTLP HTTP exporter for Volcano TLS.
func NewTLSExporter(ctx context.Context, cfg *TLSExporterConfig) (trace.SpanExporter, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		region := cfg.Region
		if region == "" {
			region = "cn-beijing"
		}
		endpoint = fmt.Sprintf("tls-%s.volces.com:4318", region)
	}

	options := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithHeaders(map[string]string{
			"Authorization": cfg.APIKey,
		}),
	}

	return otlptrace.New(ctx, otlptracehttp.NewClient(options...))
}

// NewFileExporter creates a span exporter that writes traces to a file.
func NewFileExporter(ctx context.Context, cfg Config) (trace.SpanExporter, error) {
	path := cfg.FilePath
	if path == "" {
		path = "trace.json"
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace file: %w", err)
	}
	return stdouttrace.New(stdouttrace.WithWriter(f), stdouttrace.WithPrettyPrint())
}

// NewMultiSpanExporter creates a span exporter that can export to multiple platforms simultaneously.
// It wraps the results in a TranslatedExporter.
func NewMultiSpanExporter(ctx context.Context, cfg Config) (trace.SpanExporter, error) {
	var exporters []trace.SpanExporter

	// 1. Explicit Exporter Types (Stdout/File)
	if cfg.ExporterType == ExporterStdout {
		exp, err := NewStdoutExporter()
		if err != nil {
			return nil, err
		}
		exporters = append(exporters, exp)
	} else if cfg.ExporterType == ExporterFile {
		exp, err := NewFileExporter(ctx, cfg)
		if err != nil {
			return nil, err
		}
		exporters = append(exporters, exp)
	}

	// 2. Platform Exporters (Can be multiple)
	if cfg.CozeLoop != nil && cfg.CozeLoop.APIKey != "" {
		if exp, err := NewCozeLoopExporter(ctx, cfg.CozeLoop); err == nil {
			exporters = append(exporters, exp)
		}
	}
	if cfg.ApmPlus != nil && cfg.ApmPlus.APIKey != "" {
		if exp, err := NewAPMPlusExporter(ctx, cfg.ApmPlus); err == nil {
			exporters = append(exporters, exp)
		}
	}
	if cfg.TLS != nil && cfg.TLS.APIKey != "" {
		if exp, err := NewTLSExporter(ctx, cfg.TLS); err == nil {
			exporters = append(exporters, exp)
		}
	}

	if len(exporters) == 0 {
		return nil, fmt.Errorf("no valid exporter configuration found")
	}

	var finalExp trace.SpanExporter
	if len(exporters) == 1 {
		finalExp = exporters[0]
	} else {
		finalExp = &multiSpanExporter{exporters: exporters}
	}

	return &TranslatedExporter{SpanExporter: finalExp}, nil
}

type multiSpanExporter struct {
	exporters []trace.SpanExporter
}

func (m *multiSpanExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	for _, e := range m.exporters {
		if err := e.ExportSpans(ctx, spans); err != nil {
			return err
		}
	}
	return nil
}

func (m *multiSpanExporter) Shutdown(ctx context.Context) error {
	for _, e := range m.exporters {
		if err := e.Shutdown(ctx); err != nil {
			return err
		}
	}
	return nil
}

// NewMetricReader creates one or more metric readers based on the provided configuration.
func NewMetricReader(ctx context.Context, cfg Config) ([]sdkmetric.Reader, error) {
	var readers []sdkmetric.Reader

	// 1. Explicit Types
	if cfg.ExporterType == ExporterStdout {
		if exp, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint()); err == nil {
			readers = append(readers, sdkmetric.NewPeriodicReader(exp))
		}
	} else if cfg.ExporterType == ExporterFile {
		if r, err := NewFileMetricReader(ctx, cfg); err == nil {
			readers = append(readers, r)
		}
	}

	// 2. Platforms
	if cfg.CozeLoop != nil && cfg.CozeLoop.APIKey != "" {
		if exp, err := NewCozeLoopMetricExporter(ctx, cfg.CozeLoop); err == nil {
			readers = append(readers, sdkmetric.NewPeriodicReader(exp))
		}
	}
	if cfg.ApmPlus != nil && cfg.ApmPlus.APIKey != "" {
		if exp, err := NewAPMPlusMetricExporter(ctx, cfg.ApmPlus); err == nil {
			readers = append(readers, sdkmetric.NewPeriodicReader(exp))
		}
	}
	if cfg.TLS != nil && cfg.TLS.APIKey != "" {
		if exp, err := NewTLSMetricExporter(ctx, cfg.TLS); err == nil {
			readers = append(readers, sdkmetric.NewPeriodicReader(exp))
		}
	}

	if len(readers) == 0 {
		return nil, fmt.Errorf("no valid metric configuration found")
	}
	return readers, nil
}

// NewCozeLoopMetricExporter creates an OTLP Metric exporter for CozeLoop.
func NewCozeLoopMetricExporter(ctx context.Context, cfg *CozeLoopConfig) (sdkmetric.Exporter, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "api.coze.cn"
	}
	// CozeLoop usually uses HTTP/HTTPS
	options := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(endpoint),
		otlpmetrichttp.WithHeaders(map[string]string{
			"Authorization": "Bearer " + cfg.APIKey,
			"X-Coze-Space":  cfg.ServiceName,
		}),
	}
	if !strings.HasPrefix(endpoint, "http") {
		options = append(options, otlpmetrichttp.WithInsecure())
	}
	return otlpmetrichttp.New(ctx, options...)
}

// NewAPMPlusMetricExporter creates an OTLP Metric exporter for APMPlus.
// Supports automatic gRPC (4317) detection.
func NewAPMPlusMetricExporter(ctx context.Context, cfg *ApmPlusConfig) (sdkmetric.Exporter, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "apmplus-cn-beijing.volces.com:4317"
	}

	// Heuristic: if port 4317 is explicitly mentioned or endpoint implies gRPC, use gRPC.
	// Otherwise default to HTTP (4318) or whatever is configured.
	if strings.Contains(endpoint, ":4317") {
		options := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(endpoint),
			otlpmetricgrpc.WithHeaders(map[string]string{
				"Authorization": cfg.APIKey,
			}),
			otlpmetricgrpc.WithInsecure(), // Usually internal/VPC or explicit
		}
		return otlpmetricgrpc.New(ctx, options...)
	}

	// Default to HTTP
	options := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(endpoint),
		otlpmetrichttp.WithHeaders(map[string]string{
			"Authorization": cfg.APIKey,
		}),
		otlpmetrichttp.WithInsecure(),
	}
	return otlpmetrichttp.New(ctx, options...)
}

// NewTLSMetricExporter creates an OTLP Metric exporter for Volcano TLS.
func NewTLSMetricExporter(ctx context.Context, cfg *TLSExporterConfig) (sdkmetric.Exporter, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		region := cfg.Region
		if region == "" {
			region = "cn-beijing"
		}
		endpoint = fmt.Sprintf("tls-%s.volces.com:4318", region)
	}

	options := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(endpoint),
		otlpmetrichttp.WithHeaders(map[string]string{
			"Authorization": cfg.APIKey,
		}),
	}
	return otlpmetrichttp.New(ctx, options...)
}

// NewFileMetricReader creates a metric reader that writes metrics to a file.
func NewFileMetricReader(ctx context.Context, cfg Config) (sdkmetric.Reader, error) {
	path := cfg.FilePath
	if path == "" {
		path = "metrics.json" // Different default than traces
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric file: %w", err)
	}

	// Use standard JSON encoder which satisfies stdoutmetric.Encoder interface
	enc := json.NewEncoder(f)
	enc.SetIndent("", "\t")

	exp, err := stdoutmetric.New(stdoutmetric.WithEncoder(enc))
	if err != nil {
		return nil, err
	}
	return sdkmetric.NewPeriodicReader(exp), nil
}
