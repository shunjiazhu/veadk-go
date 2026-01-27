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
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// SpanEnrichmentProcessor implements sdktrace.SpanProcessor.
// It enriches Google ADK internal spans with standard GenAI semantic conventions and
// platform-specific attributes for CozeLoop, APMPlus, and TLS platforms.
type SpanEnrichmentProcessor struct{}

func (p *SpanEnrichmentProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	name := s.Name()

	// Capture common attributes
	SetCommonAttributes(parent, s)

	// Enrich based on span type
	switch {
	case name == SpanCallLLM:
		SetLLMAttributes(s)
	case strings.HasPrefix(name, SpanExecuteTool):
		toolName := ""
		if parts := strings.SplitN(name, " ", 2); len(parts) == 2 {
			toolName = parts[1]
		}
		SetToolAttributes(s, toolName)
	case name == SpanInvokeAgent || strings.HasPrefix(name, SpanInvokeAgent+" "):
		agentName := FallbackAgentName
		if parts := strings.SplitN(name, " ", 2); len(parts) == 2 {
			agentName = parts[1]
		}
		SetAgentAttributes(s, agentName)
	case name == SpanInvocation || name == "Run":
		SetWorkflowAttributes(s)
	}
}

func (p *SpanEnrichmentProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	spanName := s.Name()
	elapsed := s.EndTime().Sub(s.StartTime()).Seconds()
	attrs := s.Attributes()

	// Convert trace attributes to metric attributes
	var metricAttrs []attribute.KeyValue
	var modelName string
	for _, kv := range attrs {
		// Map specific trace attributes to metric dimensions
		if kv.Key == SpanAttrGenAIRequestModelKey {
			modelName = kv.Value.AsString()
			metricAttrs = append(metricAttrs, attribute.String(MetricAttrGenAIRequestModelKey, modelName))
		}
	}

	// Common attributes for APMPlus
	apmplusAttrs := append(metricAttrs,
		attribute.String("gen_ai_system", "volcengine"),
		attribute.String("server_address", "api.volcengine.com"),
	)

	if spanName == SpanCallLLM {
		// Add LLM-specific attributes
		llmAttrs := append(apmplusAttrs,
			attribute.String("gen_ai_operation_name", "chat"),
			attribute.String("gen_ai_operation_type", "llm"),
			attribute.String("stream", "false"),
		)

		// Add gen_ai_response_model if available
		if modelName != "" {
			llmAttrs = append(llmAttrs, attribute.String("gen_ai_response_model", modelName))
		}

		// Record operation duration
		RecordOperationDuration(context.Background(), elapsed, metricAttrs...)

		// Record LLM invocation count
		RecordLLMInvocation(context.Background(), llmAttrs...)

		// Check for exceptions
		if s.Status().Code != codes.Ok {
			RecordChatException(context.Background(), llmAttrs...)
		}

		// Record token usage if available in attributes
		var input, output int64
		for _, kv := range attrs {
			switch kv.Key {
			case SpanAttrGenAIUsageInputTokensKey, SpanAttrGenAIResponsePromptTokenCountKey:
				input = kv.Value.AsInt64()
			case SpanAttrGenAIUsageOutputTokensKey, SpanAttrGenAIResponseCandidatesTokenCountKey:
				output = kv.Value.AsInt64()
			}
		}

		if input > 0 || output > 0 {
			RecordTokenUsage(context.Background(), input, output, llmAttrs...)
		}

		// Record APMPlus span latency
		RecordAPMPlusSpanLatency(context.Background(), elapsed, llmAttrs...)

	} else if strings.HasPrefix(spanName, SpanExecuteTool) {
		// Get tool name from attributes (most reliable source)
		toolName := ""
		if parts := strings.SplitN(spanName, " ", 2); len(parts) == 2 {
			toolName = parts[1]
		}

		// Add tool-specific attributes
		toolAttrs := append(metricAttrs,
			attribute.String("gen_ai_operation_name", toolName),
			attribute.String("gen_ai_operation_type", "tool"),
			attribute.String("gen_ai_operation_backend", ""), // Default empty
		)

		// Record operation duration
		RecordOperationDuration(context.Background(), elapsed, metricAttrs...)

		// Record APMPlus span latency
		RecordAPMPlusSpanLatency(context.Background(), elapsed, toolAttrs...)

		// Estimate tool token usage based on text length (like Python does)
		var toolInput, toolOutput string
		for _, kv := range attrs {
			switch kv.Key {
			case SpanAttrGenAIToolInputKey:
				toolInput = kv.Value.AsString()
			case SpanAttrGenAIToolOutputKey:
				toolOutput = kv.Value.AsString()
			}
			if toolInput != "" && toolOutput != "" {
				break
			}
		}

		if toolInput != "" || toolOutput != "" {
			RecordAPMPlusToolTokenUsage(context.Background(), int64(len(toolInput)/4), int64(len(toolOutput)/4), toolAttrs...)
		}

	} else if spanName == SpanInvokeAgent || spanName == SpanInvocation {
		// Record operation duration for agent spans
		RecordOperationDuration(context.Background(), elapsed, metricAttrs...)

		// Record APMPlus span latency for agent spans
		agentAttrs := append(apmplusAttrs,
			attribute.String("gen_ai_operation_name", spanName),
			attribute.String("gen_ai_operation_type", "agent"),
		)
		RecordAPMPlusSpanLatency(context.Background(), elapsed, agentAttrs...)
	}
}

func (p *SpanEnrichmentProcessor) Shutdown(ctx context.Context) error {
	return nil
}

func (p *SpanEnrichmentProcessor) ForceFlush(ctx context.Context) error {
	return nil
}
