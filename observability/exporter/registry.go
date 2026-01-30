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
	"sync"

	"go.opentelemetry.io/otel/trace"
)

var (
	// agentSpanMap tracks the current active 'invoke_agent' span context per TraceID.
	// This is used by the exporter to fix the hierarchy of ADK internal spans.
	agentSpanMap   = make(map[trace.TraceID]trace.SpanContext)
	agentSpanMutex sync.RWMutex
)

// RegisterAgentSpanContext registers an active 'invoke_agent' span context for a given TraceID.
func RegisterAgentSpanContext(traceID trace.TraceID, sc trace.SpanContext) {
	agentSpanMutex.Lock()
	defer agentSpanMutex.Unlock()
	agentSpanMap[traceID] = sc
}

// UnregisterAgentSpanContext removes the 'invoke_agent' span context for a given TraceID.
func UnregisterAgentSpanContext(traceID trace.TraceID) {
	agentSpanMutex.Lock()
	defer agentSpanMutex.Unlock()
	delete(agentSpanMap, traceID)
}

// GetAgentSpanContext retrieves the active 'invoke_agent' span context for a given TraceID.
func GetAgentSpanContext(traceID trace.TraceID) (trace.SpanContext, bool) {
	agentSpanMutex.RLock()
	defer agentSpanMutex.RUnlock()
	sc, ok := agentSpanMap[traceID]
	return sc, ok
}
