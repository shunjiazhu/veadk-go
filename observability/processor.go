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

package observability

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
)

// EnrichmentProcessor implements sdktrace.SpanProcessor.
// It enriches Google ADK internal spans with standard GenAI and platform attributes.
type EnrichmentProcessor struct{}

func (p *EnrichmentProcessor) OnStart(parent context.Context, s trace.ReadWriteSpan) {
	name := s.Name()

	// Capture common attributes from context
	if sessionId := GetSessionId(parent); sessionId != "" {
		s.SetAttributes(attribute.String(AttrSessionId, sessionId))
	}
	if userId := GetUserId(parent); userId != "" {
		s.SetAttributes(attribute.String(AttrUserId, userId))
	}
	if appName := GetAppName(parent); appName != "" {
		s.SetAttributes(attribute.String(AttrAppName, appName))
	}

	// Standard System/Platform attributes
	s.SetAttributes(
		attribute.String(AttrGenAISystem, ValGenAISystem),
		attribute.String(AttrCozeloopReportSource, ValCozeloopReportSource),
	)

	// Enrich based on span type
	if name == SpanCallLLM {
		s.SetAttributes(
			attribute.String(AttrGenAISpanKind, "llm"),
			attribute.String(AttrGenAIOperationName, "chat"),
		)
	} else if strings.HasPrefix(name, SpanExecuteTool) {
		s.SetAttributes(
			attribute.String(AttrGenAISpanKind, "tool"),
			attribute.String(AttrGenAIOperationName, "execute_tool"),
		)
		// Extract tool name from span name if possible: "execute_tool <name>"
		if parts := strings.SplitN(name, " ", 2); len(parts) == 2 {
			s.SetAttributes(attribute.String(AttrGenAIToolName, parts[1]))
		}
	} else if name == SpanInvokeAgent || strings.HasPrefix(name, SpanInvokeAgent+" ") {
		// ADK might use "invoke_agent <name>"
		if parts := strings.SplitN(name, " ", 2); len(parts) == 2 {
			s.SetAttributes(attribute.String(AttrGenAIAgentName, parts[1]))
			s.SetAttributes(attribute.String(AttrAgentName, parts[1]))
		}
	} else if name == "Run" {
		// ADK high-level runner
		s.SetAttributes(
			attribute.String(AttrGenAIOperationName, "invocation"),
		)
	}
}

func (p *EnrichmentProcessor) OnEnd(s trace.ReadOnlySpan) {
	name := s.Name()
	elapsed := s.EndTime().Sub(s.StartTime()).Seconds()
	attrs := s.Attributes()

	// Convert trace attributes to metric attributes
	var metricAttrs []attribute.KeyValue
	for _, kv := range attrs {
		// Map specific trace attributes to metric dimensions
		if kv.Key == AttrGenAIRequestModel || kv.Key == "gen_ai.request.model" {
			metricAttrs = append(metricAttrs, attribute.String(AttrGenAIRequestModel, kv.Value.AsString()))
		}
	}

	if name == SpanCallLLM {
		RecordOperationDuration(context.Background(), elapsed, metricAttrs...)

		// Record token usage if available in attributes
		var input, output int64
		for _, kv := range attrs {
			if kv.Key == AttrGenAIUsageInputTokens || kv.Key == "gen_ai.response.prompt_token_count" {
				input = kv.Value.AsInt64()
			} else if kv.Key == AttrGenAIUsageOutputTokens || kv.Key == "gen_ai.response.candidates_token_count" {
				output = kv.Value.AsInt64()
			}

			// Map ADK JSON attributes to standard fields if standard ones are missing
			if kv.Key == "gcp.vertex.agent.llm_request" {
				s.(trace.ReadWriteSpan).SetAttributes(attribute.String(AttrGenAIInputValue, kv.Value.AsString()))
			} else if kv.Key == "gcp.vertex.agent.llm_response" {
				s.(trace.ReadWriteSpan).SetAttributes(attribute.String(AttrGenAIOutputValue, kv.Value.AsString()))
			}
		}
		if input > 0 || output > 0 {
			RecordTokenUsage(context.Background(), input, output, metricAttrs...)
		}
	} else if strings.HasPrefix(name, SpanExecuteTool) {
		RecordOperationDuration(context.Background(), elapsed, metricAttrs...)
	}
}

func (p *EnrichmentProcessor) Shutdown(ctx context.Context) error   { return nil }
func (p *EnrichmentProcessor) ForceFlush(ctx context.Context) error { return nil }
