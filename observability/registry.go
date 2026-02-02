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

	// adkToVeadkInvocationMap tracks InternalRunID -> ManualInvocationSC
	adkToVeadkInvocationMap = make(map[trace.SpanID]trace.SpanContext)

	// adkToVeadkInvocateAgentMap tracks InternalAgentID -> ManualAgentSC
	adkToVeadkInvocateAgentMap = make(map[trace.SpanID]trace.SpanContext)

	// adkToVeadkLLMMap tracks InternalLLMID -> ManualLLMSC
	adkToVeadkLLMMap = make(map[trace.SpanID]trace.SpanContext)

	// toolToVeadkLLMMap tracks ToolSpanID -> ManualLLMSC.
	// This is used when ADK starts tool spans as roots (missing parent context).
	toolToVeadkLLMMap = make(map[trace.SpanID]trace.SpanContext)

	// toolCallToVeadkLLMMap tracks ToolCallID (string) -> ManualLLMSC.
	// This is the most reliable way to link tool spans when context is completely lost.
	toolCallToVeadkLLMMap = make(map[string]trace.SpanContext)

	// adkTraceToVeadkTraceMap tracks InternalTraceID -> ManualTraceID.
	// Once a single span in a trace is matched (e.g. via ToolCallID), we can align the entire trace.
	adkTraceToVeadkTraceMap = make(map[trace.TraceID]trace.TraceID)

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
	adkToVeadkInvocationMap[internalID] = manualSC
	activeInvocationSpans[manualSC.SpanID()] = manualSpan
}

// unregisterRunMapping removes run-related mappings.
func unregisterRunMapping(internalID trace.SpanID, manualSpanID trace.SpanID) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	delete(adkToVeadkInvocationMap, internalID)
	delete(activeInvocationSpans, manualSpanID)
}

// registerAgentMapping links ADK's internal agent span to our manual agent span.
func registerAgentMapping(internalID trace.SpanID, manualSC trace.SpanContext) {
	if !internalID.IsValid() || !manualSC.IsValid() {
		return
	}
	registryMutex.Lock()
	defer registryMutex.Unlock()
	adkToVeadkInvocateAgentMap[internalID] = manualSC
}

// unregisterAgentMapping removes agent mapping.
func unregisterAgentMapping(internalID trace.SpanID) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	delete(adkToVeadkInvocateAgentMap, internalID)
}

// registerLLMMapping links ADK's internal LLM span to our manual LLM span.
func registerLLMMapping(internalID trace.SpanID, manualSC trace.SpanContext) {
	if !internalID.IsValid() || !manualSC.IsValid() {
		return
	}
	registryMutex.Lock()
	defer registryMutex.Unlock()
	adkToVeadkLLMMap[internalID] = manualSC
}

// unregisterLLMMapping removes LLM mapping.
func unregisterLLMMapping(internalID trace.SpanID) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	delete(adkToVeadkLLMMap, internalID)
}

// registerToolMapping links a tool span (started by ADK) to its manual parent (LLM call).
func registerToolMapping(toolSpanID trace.SpanID, manualParentSC trace.SpanContext) {
	if !toolSpanID.IsValid() || !manualParentSC.IsValid() {
		return
	}
	registryMutex.Lock()
	defer registryMutex.Unlock()
	toolToVeadkLLMMap[toolSpanID] = manualParentSC
}

// unregisterToolMapping removes tool mapping.
func unregisterToolMapping(toolSpanID trace.SpanID) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	delete(toolToVeadkLLMMap, toolSpanID)
}

// registerToolCallMapping links a logical tool call ID to its parent LLM span context.
func registerToolCallMapping(toolCallID string, manualParentSC trace.SpanContext) {
	if toolCallID == "" || !manualParentSC.IsValid() {
		return
	}
	registryMutex.Lock()
	defer registryMutex.Unlock()
	toolCallToVeadkLLMMap[toolCallID] = manualParentSC
}

// unregisterToolCallMapping removes tool call mapping.
func unregisterToolCallMapping(toolCallID string) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	delete(toolCallToVeadkLLMMap, toolCallID)
}

// getManualParentContext finds the manual replacement for an internal parent span ID.
func getManualParentContext(internalParentID trace.SpanID) (trace.SpanContext, bool) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	// Check LLM mappings (for tools that have a valid internal parent)
	if sc, ok := adkToVeadkLLMMap[internalParentID]; ok {
		return sc, true
	}
	// Check Agent mappings (for LLM calls)
	if sc, ok := adkToVeadkInvocateAgentMap[internalParentID]; ok {
		return sc, true
	}
	// Check Run mappings (for Agent calls)
	if sc, ok := adkToVeadkInvocationMap[internalParentID]; ok {
		return sc, true
	}
	return trace.SpanContext{}, false
}

// getManualParentContextForTool finds the manual parent for a tool span by its OWN ID.
func getManualParentContextForTool(toolSpanID trace.SpanID) (trace.SpanContext, bool) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	sc, ok := toolToVeadkLLMMap[toolSpanID]
	return sc, ok
}

// getManualParentContextByToolCallID finds the manual parent for a tool span by its logical ToolCallID.
func getManualParentContextByToolCallID(toolCallID string) (trace.SpanContext, bool) {
	if toolCallID == "" {
		return trace.SpanContext{}, false
	}
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	sc, ok := toolCallToVeadkLLMMap[toolCallID]
	return sc, ok
}

// registerTraceMapping records a mapping from an internal TraceID to a manual TraceID.
func registerTraceMapping(internalTID trace.TraceID, manualTID trace.TraceID) {
	if !internalTID.IsValid() || !manualTID.IsValid() {
		return
	}
	registryMutex.Lock()
	defer registryMutex.Unlock()
	adkTraceToVeadkTraceMap[internalTID] = manualTID
}

// getManualTraceID finds the manual TraceID for an internal TraceID.
func getManualTraceID(internalTID trace.TraceID) (trace.TraceID, bool) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	tid, ok := adkTraceToVeadkTraceMap[internalTID]
	return tid, ok
}

// unregisterAllForTrace cleans up all mappings related to an internal TraceID.
func unregisterAllForTrace(internalTID trace.TraceID) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	delete(adkTraceToVeadkTraceMap, internalTID)
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
