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

	"go.opentelemetry.io/otel/sdk/trace"
)

// VeADKSpanProcessor is a custom SpanProcessor that enriches spans with standard GenAI attributes
// and handles span lifecycle events for hierarchy management.
type VeADKSpanProcessor struct {
	trace.SpanProcessor
}

func NewVeADKSpanProcessor() *VeADKSpanProcessor {
	return &VeADKSpanProcessor{}
}

func (p *VeADKSpanProcessor) OnStart(ctx context.Context, s trace.ReadWriteSpan) {
	name := s.Name()
	sc := s.SpanContext()
	traceID := sc.TraceID()

	// 1. If it's the 'invocation' span, register as root
	if name == "invocation" {
		RegisterInvocationSpan(traceID, s)
	}

	// 2. If it's the 'invoke_agent' span, register its context for re-parenting children
	if name == "invoke_agent" || strings.HasPrefix(name, "invoke_agent:") {
		RegisterAgentSpanContext(traceID, sc)
	}

	// 3. (Optional) Perform light enrichment if needed at start time.
	// Most attribute mapping is deferred to the Translator in ExportSpans
	// to handle attributes added late in the span lifecycle.
}

func (p *VeADKSpanProcessor) OnEnd(s trace.ReadOnlySpan) {
}

func (p *VeADKSpanProcessor) Shutdown(ctx context.Context) error {
	return nil
}

func (p *VeADKSpanProcessor) ForceFlush(ctx context.Context) error {
	return nil
}
