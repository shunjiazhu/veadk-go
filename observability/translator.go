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
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/trace"
)

var (
	// ADKAttributeKeyMap defines the mapping from Google ADK-internal keys to GenAI standard keys.
	ADKAttributeKeyMap = map[string]string{
		"gcp.vertex.agent.llm_request":   "input.value",
		"gcp.vertex.agent.llm_response":  "output.value",
		"gcp.vertex.agent.session_id":    "gen_ai.session.id",
		"gcp.vertex.agent.invocation_id": "gen_ai.invocation.id",
		"gcp.vertex.agent.event_id":      "gen_ai.event.id",
		"gen_ai.operation.name":          "gen_ai.request.type",
	}

	// ADKTargetKeyAliases defines aliases for target standard keys that prevent redundant mapping.
	ADKTargetKeyAliases = map[string][]string{
		"input.value":  {"gen_ai.input.value"},
		"output.value": {"gen_ai.output.value"},
	}
)

// TranslatedExporter wraps a SpanExporter and remaps ADK attributes to VeADK standard fields.
type TranslatedExporter struct {
	trace.SpanExporter
}

func (e *TranslatedExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	translated := make([]trace.ReadOnlySpan, len(spans))
	for i, s := range spans {
		translated[i] = &translatedSpan{ReadOnlySpan: s}
	}
	return e.SpanExporter.ExportSpans(ctx, translated)
}

// translatedSpan wraps a ReadOnlySpan and intercepts calls to Attributes().
type translatedSpan struct {
	trace.ReadOnlySpan
}

func (p *translatedSpan) Attributes() []attribute.KeyValue {
	attrs := p.ReadOnlySpan.Attributes()

	presentKeys := make(map[string]struct{}, len(attrs)+len(ADKAttributeKeyMap))
	sourceValues := make(map[string]attribute.Value)

	newAttrs := make([]attribute.KeyValue, 0, len(attrs)+len(ADKAttributeKeyMap))

	for i := range attrs {
		kv := attrs[i]
		key := string(kv.Key)

		if _, isSource := ADKAttributeKeyMap[key]; isSource {
			sourceValues[key] = kv.Value
		}

		if strings.HasPrefix(key, "gcp.vertex.agent.") {
			continue
		}

		presentKeys[key] = struct{}{}

		if key == AttrGenAISystem && kv.Value.AsString() == "gcp.vertex.agent" {
			kv = attribute.String(AttrGenAISystem, ValCozeloopReportSource)
		}

		newAttrs = append(newAttrs, kv)
	}

	for sourceKey, targetKey := range ADKAttributeKeyMap {
		isAlreadyPresent := false
		if _, ok := presentKeys[targetKey]; ok {
			isAlreadyPresent = true
		} else {
			for _, alias := range ADKTargetKeyAliases[targetKey] {
				if _, ok := presentKeys[alias]; ok {
					isAlreadyPresent = true
					break
				}
			}
		}

		if !isAlreadyPresent {
			if val, ok := sourceValues[sourceKey]; ok {
				newAttrs = append(newAttrs, attribute.KeyValue{Key: attribute.Key(targetKey), Value: val})
				presentKeys[targetKey] = struct{}{}
			}
		}
	}

	return newAttrs
}

func (p *translatedSpan) InstrumentationScope() instrumentation.Scope {
	scope := p.ReadOnlySpan.InstrumentationScope()
	if scope.Name == "gcp.vertex.agent" || scope.Name == "veadk" {
		scope.Name = "openinference.instrumentation.veadk"
		scope.Version = Version
	}
	return scope
}

func (p *translatedSpan) InstrumentationLibrary() instrumentation.Scope {
	return p.InstrumentationScope()
}
