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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSpanEnrichmentProcessor(t *testing.T) {
	// 1. Setup Metrics to verify side effects of OnEnd
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	// Initialize instruments into global slice for this test
	initializeInstruments(mp.Meter("processor-test"))

	// 2. Setup Tracer with Processor
	exporter := tracetest.NewInMemoryExporter()
	processor := &SpanEnrichmentProcessor{EnableMetrics: true}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(processor),
		sdktrace.WithSyncer(exporter),
	)
	tracer := tp.Tracer("test-tracer")

	ctx := context.Background()

	t.Run("LLM Span", func(t *testing.T) {
		_, span := tracer.Start(ctx, SpanCallLLM)
		// Add usage attributes
		span.SetAttributes(
			attribute.Int64(AttrGenAIUsageInputTokens, 100),
			attribute.Int64(AttrGenAIUsageOutputTokens, 200),
		)
		span.End()

		spans := exporter.GetSpans()
		if assert.Len(t, spans, 1) {
			s := spans[0]
			assert.Equal(t, SpanCallLLM, s.Name)
			// Check enriched attributes
			var foundKind, foundOp bool
			for _, a := range s.Attributes {
				if a.Key == AttrGenAISpanKind && a.Value.AsString() == SpanKindLLM {
					foundKind = true
				}
				if a.Key == AttrGenAIOperationName && a.Value.AsString() == "chat" {
					foundOp = true
				}
			}
			assert.True(t, foundKind, "GenAISpanKind should be LLM")
			assert.True(t, foundOp, "GenAIOperationName should be chat")
		}
		exporter.Reset()

		// Verify Metrics
		var rm metricdata.ResourceMetrics
		err := reader.Collect(ctx, &rm)
		assert.NoError(t, err)

		var foundToken, foundDuration, foundChatCount, foundAPMLatency bool
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == MetricNameTokenUsage {
					foundToken = true
				}
				if m.Name == MetricNameOperationDuration {
					foundDuration = true
				}
				if m.Name == MetricNameChatCount {
					foundChatCount = true
				}
				if m.Name == MetricNameAPMPlusSpanLatency {
					foundAPMLatency = true
				}
			}
		}
		assert.True(t, foundToken, "Token usage metric should be recorded")
		assert.True(t, foundDuration, "Duration metric should be recorded")
		assert.True(t, foundChatCount, "Chat count metric should be recorded")
		assert.True(t, foundAPMLatency, "APMPlus Span Latency metric should be recorded")
	})

	t.Run("Tool Span", func(t *testing.T) {
		_, span := tracer.Start(ctx, SpanExecuteTool+" my_tool")
		// Add tool input/output for token estimation (4 chars = 1 token)
		span.SetAttributes(
			attribute.String(AttrGenAIToolInput, "1234"),      // 1 token
			attribute.String(AttrGenAIToolOutput, "12345678"), // 2 tokens
		)
		span.End()

		spans := exporter.GetSpans()
		if assert.Len(t, spans, 1) {
			s := spans[0]
			// Check enriched attributes
			var foundToolName bool
			for _, a := range s.Attributes {
				if a.Key == AttrGenAIToolName && a.Value.AsString() == "my_tool" {
					foundToolName = true
				}
			}
			assert.True(t, foundToolName, "Tool name mismatch")
		}
		exporter.Reset()

		// Verify Token Usage for Tool
		var rm metricdata.ResourceMetrics
		err := reader.Collect(ctx, &rm)
		assert.NoError(t, err)

		var foundToolToken bool
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == MetricNameAPMPlusToolTokenUsage {
					foundToolToken = true
					// Could check data points if needed, but existence is enough for now
				}
			}
		}
		assert.True(t, foundToolToken, "APMPlus Tool Token Usage should be recorded")
	})

	t.Run("Agent Span", func(t *testing.T) {
		_, span := tracer.Start(ctx, SpanInvokeAgent+" my_agent")
		span.SetAttributes(
			attribute.Int64(AttrGenAIUsageInputTokens, 50),
			attribute.Int64(AttrGenAIUsageOutputTokens, 100),
		)
		span.End()

		spans := exporter.GetSpans()
		if assert.Len(t, spans, 1) {
			s := spans[0]
			var foundAgentName bool
			for _, a := range s.Attributes {
				if a.Key == AttrGenAIAgentName && a.Value.AsString() == "my_agent" {
					foundAgentName = true
				}
			}
			assert.True(t, foundAgentName, "Agent name mismatch")
		}
		exporter.Reset()

		// Verify Metrics for Agent
		var rm metricdata.ResourceMetrics
		err := reader.Collect(ctx, &rm)
		assert.NoError(t, err)

		var foundToken, foundDuration bool
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == MetricNameTokenUsage {
					foundToken = true
				}
				if m.Name == MetricNameOperationDuration {
					foundDuration = true
				}
			}
		}
		assert.True(t, foundToken, "Token usage metric should be recorded for Agent span")
		assert.True(t, foundDuration, "Duration metric should be recorded for Agent span")
	})

	t.Run("Lifecycle", func(t *testing.T) {
		assert.NoError(t, processor.ForceFlush(ctx))
		assert.NoError(t, processor.Shutdown(ctx))
	})
}
