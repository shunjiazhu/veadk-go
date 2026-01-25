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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTranslatedSpan_Attributes(t *testing.T) {
	// 1. Setup mock span with ADK internal attributes
	mockSpan := &tracetest.SpanStub{
		Attributes: []attribute.KeyValue{
			attribute.String("gcp.vertex.agent.llm_request", "fake-request"),
			attribute.String("gcp.vertex.agent.llm_response", "fake-response"),
			attribute.String("gcp.vertex.agent.session_id", "session-123"),
			attribute.String("gen_ai.system", "gcp.vertex.agent"), // Should be translated to veadk
			attribute.String("other.attr", "keep-me"),
		},
		InstrumentationScope: instrumentation.Scope{
			Name: "gcp.vertex.agent",
		},
	}

	ts := &translatedSpan{ReadOnlySpan: mockSpan.Snapshot()}
	attrs := ts.Attributes()

	// 2. Verify translations
	attrMap := make(map[string]string)
	for _, kv := range attrs {
		attrMap[string(kv.Key)] = kv.Value.AsString()
	}

	// ADK internal attributes should be filtered out
	assert.NotContains(t, attrMap, "gcp.vertex.agent.llm_request")

	// Mapped standard attributes should be present
	assert.Equal(t, "fake-request", attrMap["input.value"])
	assert.Equal(t, "fake-response", attrMap["output.value"])
	assert.Equal(t, "session-123", attrMap["gen_ai.session.id"])

	// System name correction
	assert.Equal(t, "veadk", attrMap["gen_ai.system"])

	// Unrelated attributes should be kept
	assert.Equal(t, "keep-me", attrMap["other.attr"])
}

func TestTranslatedSpan_InstrumentationScope(t *testing.T) {
	mockSpan := &tracetest.SpanStub{
		InstrumentationScope: instrumentation.Scope{
			Name:    "gcp.vertex.agent",
			Version: "1.0.0",
		},
	}

	ts := &translatedSpan{ReadOnlySpan: mockSpan.Snapshot()}
	scope := ts.InstrumentationScope()

	assert.Equal(t, "openinference.instrumentation.veadk", scope.Name)
	assert.Equal(t, "1.0.0", scope.Version) // Keeps original version in this layer
}

func TestTranslatedExporter_ExportSpans(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	te := &TranslatedExporter{SpanExporter: exporter}

	mockSpan := &tracetest.SpanStub{
		Name: "test-span",
		Attributes: []attribute.KeyValue{
			attribute.String("gcp.vertex.agent.llm_request", "data"),
		},
	}

	err := te.ExportSpans(context.Background(), []trace.ReadOnlySpan{mockSpan.Snapshot()})
	assert.NoError(t, err)

	spans := exporter.GetSpans()
	assert.Len(t, spans, 1)
}
