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
	agentSpanMap   = make(map[trace.TraceID]trace.SpanContext)
	// invocationSpanMap tracks the root 'invocation' span context per TraceID.
	invocationSpanMap = make(map[trace.TraceID]trace.SpanContext)
	// activeInvocationSpans tracks the actual span objects to ensure they can be ended on shutdown.
	activeInvocationSpans = make(map[trace.TraceID]trace.Span)
	// lastLLMSpanMap tracks the last 'call_llm' span context per TraceID for re-parenting tools.
	lastLLMSpanMap = make(map[trace.TraceID]trace.SpanContext)
	registryMutex     sync.RWMutex
)

// RegisterAgentSpanContext registers an active 'invoke_agent' span context for a given TraceID.
func RegisterAgentSpanContext(traceID trace.TraceID, sc trace.SpanContext) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	agentSpanMap[traceID] = sc
}

// UnregisterAgentSpanContext removes the 'invoke_agent' span context for a given TraceID.
func UnregisterAgentSpanContext(traceID trace.TraceID) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	delete(agentSpanMap, traceID)
}

// GetAgentSpanContext retrieves the active 'invoke_agent' span context for a given TraceID.
func GetAgentSpanContext(traceID trace.TraceID) (trace.SpanContext, bool) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	sc, ok := agentSpanMap[traceID]
	return sc, ok
}

// RegisterLLMSpanContext registers the last 'call_llm' span context for a given TraceID.
func RegisterLLMSpanContext(traceID trace.TraceID, sc trace.SpanContext) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	lastLLMSpanMap[traceID] = sc
}

// UnregisterLLMSpanContext removes the last 'call_llm' span context for a given TraceID.
func UnregisterLLMSpanContext(traceID trace.TraceID) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	delete(lastLLMSpanMap, traceID)
}

// GetLLMSpanContext retrieves the last 'call_llm' span context for a given TraceID.
func GetLLMSpanContext(traceID trace.TraceID) (trace.SpanContext, bool) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	sc, ok := lastLLMSpanMap[traceID]
	return sc, ok
}

// RegisterInvocationSpan registers the root 'invocation' span for a given TraceID.
func RegisterInvocationSpan(traceID trace.TraceID, span trace.Span) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	invocationSpanMap[traceID] = span.SpanContext()
	activeInvocationSpans[traceID] = span
}

// UnregisterInvocationSpan removes the 'invocation' span for a given TraceID.
func UnregisterInvocationSpan(traceID trace.TraceID) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	delete(invocationSpanMap, traceID)
	delete(activeInvocationSpans, traceID)
}

// EndAllInvocationSpans ends all currently active invocation spans.
// This is used during graceful shutdown to ensure the root spans are closed and flushed.
func EndAllInvocationSpans() {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	for id, span := range activeInvocationSpans {
		if span.IsRecording() {
			span.End()
		}
		delete(activeInvocationSpans, id)
		delete(invocationSpanMap, id)
	}
}

// GetInvocationSpanContexts returns all currently registered invocation span contexts.
func GetInvocationSpanContexts() []trace.SpanContext {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	res := make([]trace.SpanContext, 0, len(invocationSpanMap))
	for _, sc := range invocationSpanMap {
		res = append(res, sc)
	}
	return res
}
