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
type SpanEnrichmentProcessor struct {
	EnableMetrics bool
}

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
	case name == "Run" || name == SpanInvocation:
		SetWorkflowAttributes(s)
	}
}

func (p *SpanEnrichmentProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	if !p.EnableMetrics {
		return
	}
	spanName := s.Name()
	elapsed := s.EndTime().Sub(s.StartTime()).Seconds()
	attrs := s.Attributes()

	// Convert trace attributes to metric attributes with Python-aligned keys (underscores)
	var metricAttrs []attribute.KeyValue
	var toolName string

	for _, kv := range attrs {
		switch kv.Key {
		case attribute.Key(AttrGenAIRequestModel):
			// Python maps request model to "gen_ai_response_model"
			metricAttrs = append(metricAttrs, attribute.String("gen_ai_response_model", kv.Value.AsString()))
		case attribute.Key(AttrGenAIOperationName):
			metricAttrs = append(metricAttrs, attribute.String("gen_ai_operation_name", kv.Value.AsString()))
		case attribute.Key(AttrGenAISpanKind):
			metricAttrs = append(metricAttrs, attribute.String("gen_ai_operation_type", kv.Value.AsString()))
		case attribute.Key(AttrGenAISystem):
			metricAttrs = append(metricAttrs, attribute.String("gen_ai_system", kv.Value.AsString()))
		case attribute.Key(AttrGenAIToolName):
			toolName = kv.Value.AsString()
		}
	}

	// Override operation name for Tool spans (Python uses tool name as operation name for metrics)
	if strings.HasPrefix(spanName, SpanExecuteTool) && toolName != "" {
		// Remove generic "execute_tool" operation name if present
		var filteredAttrs []attribute.KeyValue
		for _, kv := range metricAttrs {
			if string(kv.Key) != "gen_ai_operation_name" {
				filteredAttrs = append(filteredAttrs, kv)
			}
		}
		metricAttrs = append(filteredAttrs, attribute.String("gen_ai_operation_name", toolName))
	} else if spanName == SpanCallLLM {
		// Ensure system="volcengine" if missing (Python does this)
		hasSystem := false
		for _, kv := range metricAttrs {
			if string(kv.Key) == "gen_ai_system" {
				hasSystem = true
				break
			}
		}
		if !hasSystem {
			metricAttrs = append(metricAttrs, attribute.String("gen_ai_system", "volcengine"))
		}
	}

	if spanName == SpanCallLLM {
		RecordOperationDuration(context.Background(), elapsed, metricAttrs...)
		RecordAPMPlusSpanLatency(context.Background(), elapsed, metricAttrs...)
		RecordChatCount(context.Background(), 1, metricAttrs...)

		if s.Status().Code == codes.Error {
			metricAttrs = append(metricAttrs, attribute.String("error_type", s.Status().Description))
			RecordExceptions(context.Background(), 1, metricAttrs...)
		}

		// Record token usage if available in attributes
		var input, output int64
		var isStreaming bool

		for _, kv := range attrs {
			switch kv.Key {
			case attribute.Key(AttrGenAIUsageInputTokens), attribute.Key(AttrGenAIResponsePromptTokenCount):
				input = kv.Value.AsInt64()
			case attribute.Key(AttrGenAIUsageOutputTokens), attribute.Key(AttrGenAIResponseCandidatesTokenCount):
				output = kv.Value.AsInt64()
			case attribute.Key(AttrGenAIIsStreaming):
				isStreaming = kv.Value.AsBool()
			}
		}

		// RecordTokenUsage handles splitting input/output and adding gen_ai_token_type automatically
		if input > 0 || output > 0 {
			RecordTokenUsage(context.Background(), input, output, metricAttrs...)
		}

		if isStreaming {
			RecordStreamingTimeToGenerate(context.Background(), elapsed, metricAttrs...)
			if output > 0 {
				RecordStreamingTimePerOutputToken(context.Background(), elapsed/float64(output), metricAttrs...)
			}
		}

	} else if strings.HasPrefix(spanName, SpanExecuteTool) {
		RecordOperationDuration(context.Background(), elapsed, metricAttrs...)
		RecordAPMPlusSpanLatency(context.Background(), elapsed, metricAttrs...)

		// Record tool token usage (estimated from input/output length)
		// 1 token ~= 4 chars. Recorded separately for input and output.
		var inputChars, outputChars int64
		for _, kv := range attrs {
			if kv.Key == attribute.Key(AttrGenAIToolInput) || kv.Key == attribute.Key(AttrCozeloopInput) {
				inputChars += int64(len(kv.Value.AsString()))
			}
			if kv.Key == attribute.Key(AttrGenAIToolOutput) || kv.Key == attribute.Key(AttrCozeloopOutput) {
				outputChars += int64(len(kv.Value.AsString()))
			}
		}

		if inputChars > 0 {
			RecordAPMPlusToolTokenUsage(context.Background(), inputChars/4, append(metricAttrs, attribute.String("token_type", "input"))...)
		}
		if outputChars > 0 {
			RecordAPMPlusToolTokenUsage(context.Background(), outputChars/4, append(metricAttrs, attribute.String("token_type", "output"))...)
		}
	}
}

func (p *SpanEnrichmentProcessor) Shutdown(ctx context.Context) error {
	return nil
}

func (p *SpanEnrichmentProcessor) ForceFlush(ctx context.Context) error {
	return nil
}
