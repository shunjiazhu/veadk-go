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
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/adk/telemetry"
)

const (
	InstrumentationName = "github.com/volcengine/veadk-go"
)

func init() {
	RegisterSpanProcessor(&EnrichmentProcessor{})
}

func RegisterSpanProcessor(processor sdktrace.SpanProcessor) {
	telemetry.RegisterSpanProcessor(processor)
}

// Setup initializes the observability system by registering the exporter to both
// Google ADK's local telemetry and the global OTel TracerProvider.
func Setup(exporter sdktrace.SpanExporter, serviceName string) {
	// 1. Register to ADK local telemetry
	telemetry.RegisterSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter))

	// 2. Register to Global OTel TracerProvider
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
		sdktrace.WithSpanProcessor(&EnrichmentProcessor{}),
	)
	otel.SetTracerProvider(tp)
}

func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	tr := otel.Tracer(InstrumentationName)
	ctx, span := tr.Start(ctx, name)

	// Invert injection: If span is already present, add common attributes
	if sessionId := GetSessionId(ctx); sessionId != "" {
		span.SetAttributes(attribute.String(AttrSessionId, sessionId))
	}
	if userId := GetUserId(ctx); userId != "" {
		span.SetAttributes(attribute.String(AttrUserId, userId))
	}
	if appName := GetAppName(ctx); appName != "" {
		span.SetAttributes(attribute.String(AttrAppName, appName))
	}

	return ctx, span
}

func WithSessionId(ctx context.Context, sessionId string) context.Context {
	return context.WithValue(ctx, ContextKeySessionId, sessionId)
}

func GetSessionId(ctx context.Context) string {
	if val, ok := ctx.Value(ContextKeySessionId).(string); ok {
		return val
	}
	return ""
}

func WithUserId(ctx context.Context, userId string) context.Context {
	return context.WithValue(ctx, ContextKeyUserId, userId)
}

func GetUserId(ctx context.Context) string {
	if val, ok := ctx.Value(ContextKeyUserId).(string); ok {
		return val
	}
	return ""
}

func WithAppName(ctx context.Context, appName string) context.Context {
	return context.WithValue(ctx, ContextKeyAppName, appName)
}

func GetAppName(ctx context.Context) string {
	if val, ok := ctx.Value(ContextKeyAppName).(string); ok {
		return val
	}
	return ""
}
