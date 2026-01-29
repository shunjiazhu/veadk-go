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
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/trace"
)

var (
	// ADKAttributeKeyMap maps ADK internal attribute keys to standard GenAI keys.
	ADKAttributeKeyMap = map[string]string{
		"gcp.vertex.agent.llm_request":  "input.value",
		"gcp.vertex.agent.llm_response": "output.value",
		"gcp.vertex.agent.session_id":   "gen_ai.session.id",
		"gcp.vertex.agent.model":        "gen_ai.response.model",
		"gcp.vertex.agent.usage":        "gen_ai.usage",
	}

	// ADKTargetKeyAliases maps standard keys to their legacy or platform-specific aliases.
	ADKTargetKeyAliases = map[string][]string{
		"gen_ai.response.model": {"gcp.vertex.agent.model"},
		"gen_ai.usage":          {"gcp.vertex.agent.usage"},
	}
)

// ADKTranslatedExporter wraps a SpanExporter and remaps ADK attributes to standard fields.
type ADKTranslatedExporter struct {
	trace.SpanExporter
}

func (e *ADKTranslatedExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	translated := make([]trace.ReadOnlySpan, len(spans))
	for i, s := range spans {
		translated[i] = &translatedSpan{ReadOnlySpan: s}
	}
	return e.SpanExporter.ExportSpans(ctx, translated)
	// return e.SpanExporter.ExportSpans(ctx, spans)
}

// translatedSpan wraps a ReadOnlySpan and intercepts calls to Attributes().
type translatedSpan struct {
	trace.ReadOnlySpan
}

func (p *translatedSpan) Attributes() []attribute.KeyValue {
	attrs := p.ReadOnlySpan.Attributes()
	newAttrs := make([]attribute.KeyValue, 0, len(attrs))

	for _, kv := range attrs {
		key := string(kv.Key)

		// 1. Map ADK internal attributes if not already present in standard form
		if strings.HasPrefix(key, "gcp.vertex.agent.") {
			if targetKey, ok := ADKAttributeKeyMap[key]; ok {
				newAttrs = append(newAttrs, attribute.KeyValue{Key: attribute.Key(targetKey), Value: kv.Value})
			}
			continue
		}

		// 2. Patch gen_ai.system if needed
		if key == "gen_ai.system" && kv.Value.AsString() == "gcp.vertex.agent" {
			kv = attribute.String("gen_ai.system", "veadk")
		}

		newAttrs = append(newAttrs, kv)
	}

	return newAttrs
}

func (p *translatedSpan) InstrumentationScope() instrumentation.Scope {
	scope := p.ReadOnlySpan.InstrumentationScope()
	if scope.Name == "gcp.vertex.agent" || scope.Name == "veadk" {
		scope.Name = "openinference.instrumentation.veadk"
		// Version detection is handled in the main package to avoid repetition
	}
	return scope
}

func (p *translatedSpan) InstrumentationLibrary() instrumentation.Scope {
	return p.InstrumentationScope()
}
