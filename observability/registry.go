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
	"sync"

	"go.opentelemetry.io/otel/trace"
)

var (
	registryMutex sync.RWMutex

	// [Thin Layer] ID Mappings.
	// We map ADK internal SpanIDs to our manual plugin SpanContexts.
	// This allows precise re-parenting regardless of TraceID collisions or nesting.

	// internalToManualRunMap tracks InternalRunID -> ManualInvocationSC
	internalToManualRunMap = make(map[trace.SpanID]trace.SpanContext)

	// internalToManualAgentMap tracks InternalAgentID -> ManualAgentSC
	internalToManualAgentMap = make(map[trace.SpanID]trace.SpanContext)

	// internalToManualLLMMap tracks InternalLLMID -> ManualLLMSC
	internalToManualLLMMap = make(map[trace.SpanID]trace.SpanContext)

	// activeInvocationSpans tracks active manual invocation spans for shutdown flushing.
	activeInvocationSpans = make(map[trace.SpanID]trace.Span)
)

// registerRunMapping links ADK's internal run span to our manual invocation span.
func registerRunMapping(internalID trace.SpanID, manualSC trace.SpanContext, manualSpan trace.Span) {
	if !internalID.IsValid() || !manualSC.IsValid() {
		return
	}
	registryMutex.Lock()
	defer registryMutex.Unlock()
	internalToManualRunMap[internalID] = manualSC
	activeInvocationSpans[manualSC.SpanID()] = manualSpan
}

// unregisterRunMapping removes run-related mappings.
func unregisterRunMapping(internalID trace.SpanID, manualSpanID trace.SpanID) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	delete(internalToManualRunMap, internalID)
	delete(activeInvocationSpans, manualSpanID)
}

// registerAgentMapping links ADK's internal agent span to our manual agent span.
func registerAgentMapping(internalID trace.SpanID, manualSC trace.SpanContext) {
	if !internalID.IsValid() || !manualSC.IsValid() {
		return
	}
	registryMutex.Lock()
	defer registryMutex.Unlock()
	internalToManualAgentMap[internalID] = manualSC
}

// unregisterAgentMapping removes agent mapping.
func unregisterAgentMapping(internalID trace.SpanID) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	delete(internalToManualAgentMap, internalID)
}

// registerLLMMapping links ADK's internal LLM span to our manual LLM span.
func registerLLMMapping(internalID trace.SpanID, manualSC trace.SpanContext) {
	if !internalID.IsValid() || !manualSC.IsValid() {
		return
	}
	registryMutex.Lock()
	defer registryMutex.Unlock()
	internalToManualLLMMap[internalID] = manualSC
}

// unregisterLLMMapping removes LLM mapping.
func unregisterLLMMapping(internalID trace.SpanID) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	delete(internalToManualLLMMap, internalID)
}

// getManualParentContext finds the manual replacement for an internal parent span ID.
func getManualParentContext(internalParentID trace.SpanID) (trace.SpanContext, bool) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	// Check LLM mappings (for tools)
	if sc, ok := internalToManualLLMMap[internalParentID]; ok {
		return sc, true
	}
	// Check Agent mappings (for LLM calls)
	if sc, ok := internalToManualAgentMap[internalParentID]; ok {
		return sc, true
	}
	// Check Run mappings (for Agent calls)
	if sc, ok := internalToManualRunMap[internalParentID]; ok {
		return sc, true
	}
	return trace.SpanContext{}, false
}

// endAllInvocationSpans ends all currently active invocation spans.
func endAllInvocationSpans() {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	for id, span := range activeInvocationSpans {
		if span.IsRecording() {
			span.End()
		}
		delete(activeInvocationSpans, id)
	}
}
