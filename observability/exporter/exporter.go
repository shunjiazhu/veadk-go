// Copyright (c) 2025 Beijing Volcano Engine Technology Co., Ltd. and/or its affiliates.
//
// Licensed under the Apache License, Beijing 2.0 (the "License");
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
func NewCozeLoopExporter(ctx context.Context, cfg Config) (trace.SpanExporter, error) {
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
func NewAPMPlusExporter(ctx context.Context, cfg Config) (trace.SpanExporter, error) {
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
func NewTLSExporter(ctx context.Context, cfg Config) (trace.SpanExporter, error) {
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

// NewMetricReader creates a metric reader based on the provided configuration.
func NewMetricReader(ctx context.Context, cfg Config) (sdkmetric.Reader, error) {
	var exp sdkmetric.Exporter
	var err error

	switch cfg.ExporterType {
	case ExporterStdout:
		exp, err = stdoutmetric.New(stdoutmetric.WithPrettyPrint())
	case ExporterFile:
		return NewFileMetricReader(ctx, cfg) // File Reader needs special handling
	case ExporterCozeLoop:
		exp, err = NewCozeLoopMetricExporter(ctx, cfg)
	case ExporterAPMPlus:
		exp, err = NewAPMPlusMetricExporter(ctx, cfg)
	case ExporterTLS:
		exp, err = NewTLSMetricExporter(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported exporter type for metrics: %s", cfg.ExporterType)
	}

	if err != nil {
		return nil, err
	}
	return sdkmetric.NewPeriodicReader(exp), nil
}

// NewCozeLoopMetricExporter creates an OTLP Metric exporter for CozeLoop.
func NewCozeLoopMetricExporter(ctx context.Context, cfg Config) (sdkmetric.Exporter, error) {
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
func NewAPMPlusMetricExporter(ctx context.Context, cfg Config) (sdkmetric.Exporter, error) {
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
func NewTLSMetricExporter(ctx context.Context, cfg Config) (sdkmetric.Exporter, error) {
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
