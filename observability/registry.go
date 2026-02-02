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
	"time"

	"go.opentelemetry.io/otel/trace"
)

// TraceRegistry manages the mapping between ADK-go's spans and VeADK spans.
// It ensures thread-safe access and proper cleanup of resources.
type TraceRegistry struct {
	// adkSpanMap tracks google's adk SpanID (Run/Agent/LLM/Tool) -> VeADK SpanContext
	adkSpanMap sync.Map

	// toolCallMap tracks ToolCallID (string) -> *toolCallInfo
	// Consolidates: toolCallToVeadkLLMMap, toolInputs, toolOutputs
	toolCallMap sync.Map

	// activeInvocationSpans tracks active VeADK invocation spans for shutdown flushing.
	activeInvocationSpans sync.Map

	// adkTraceToVeadkTraceMap tracks InternalTraceID -> Associated Resources for cleanup.
	resourcesMu             sync.RWMutex
	adkTraceToVeadkTraceMap map[trace.TraceID]*traceInfos
}

type toolCallInfo struct {
	mu       sync.RWMutex
	parentSC trace.SpanContext
}

type traceInfos struct {
	veadkTraceID trace.TraceID
	spanIDs      []trace.SpanID
	toolCallIDs  []string
}

var (
	// globalRegistry is the singleton instance of TraceRegistry.
	globalRegistry = &TraceRegistry{
		adkTraceToVeadkTraceMap: make(map[trace.TraceID]*traceInfos),
	}
)

// GetRegistry returns the global TraceRegistry.
func GetRegistry() *TraceRegistry {
	return globalRegistry
}

func (r *TraceRegistry) getOrCreateTraceInfos(adkTraceID trace.TraceID) *traceInfos {
	r.resourcesMu.Lock()
	defer r.resourcesMu.Unlock()

	if res, ok := r.adkTraceToVeadkTraceMap[adkTraceID]; ok {
		return res
	}
	res := &traceInfos{}
	r.adkTraceToVeadkTraceMap[adkTraceID] = res
	return res
}

// RegisterRunMapping links ADK's internal run span to our veadk invocation span.
func (r *TraceRegistry) RegisterRunMapping(adkSpanID trace.SpanID, adkTraceID trace.TraceID, veadkSC trace.SpanContext, veadkSpan trace.Span) {
	if !adkSpanID.IsValid() || !veadkSC.IsValid() {
		return
	}
	r.adkSpanMap.Store(adkSpanID, veadkSC)
	r.activeInvocationSpans.Store(veadkSC.SpanID(), veadkSpan)

	if adkTraceID.IsValid() {
		res := r.getOrCreateTraceInfos(adkTraceID)
		r.resourcesMu.Lock()
		res.spanIDs = append(res.spanIDs, adkSpanID)
		res.veadkTraceID = veadkSC.TraceID()
		r.resourcesMu.Unlock()
	}
}

// RegisterAgentMapping links ADK's internal agent span to our veadk agent span.
func (r *TraceRegistry) RegisterAgentMapping(adkSpanID trace.SpanID, adkTraceID trace.TraceID, veadkSC trace.SpanContext) {
	if !adkSpanID.IsValid() || !veadkSC.IsValid() {
		return
	}
	r.adkSpanMap.Store(adkSpanID, veadkSC)

	if adkTraceID.IsValid() {
		res := r.getOrCreateTraceInfos(adkTraceID)
		r.resourcesMu.Lock()
		res.spanIDs = append(res.spanIDs, adkSpanID)
		r.resourcesMu.Unlock()
	}
}

// RegisterLLMMapping links ADK's internal LLM span to our veadk LLM span.
func (r *TraceRegistry) RegisterLLMMapping(adkSpanID trace.SpanID, adkTraceID trace.TraceID, veadkSC trace.SpanContext) {
	if !adkSpanID.IsValid() || !veadkSC.IsValid() {
		return
	}
	r.adkSpanMap.Store(adkSpanID, veadkSC)

	if adkTraceID.IsValid() {
		res := r.getOrCreateTraceInfos(adkTraceID)
		r.resourcesMu.Lock()
		res.spanIDs = append(res.spanIDs, adkSpanID)
		r.resourcesMu.Unlock()
	}
}

// RegisterToolMapping links a tool span (started by ADK) to its veadk parent (LLM call).
func (r *TraceRegistry) RegisterToolMapping(toolSpanID trace.SpanID, veadkParentSC trace.SpanContext) {
	if !toolSpanID.IsValid() || !veadkParentSC.IsValid() {
		return
	}
	r.adkSpanMap.Store(toolSpanID, veadkParentSC)
}

func (r *TraceRegistry) getOrCreateToolCallInfo(toolCallID string) *toolCallInfo {
	val, loaded := r.toolCallMap.LoadOrStore(toolCallID, &toolCallInfo{})
	if !loaded {
		// New entry
	}
	return val.(*toolCallInfo)
}

// RegisterToolCallMapping links a logical tool call ID to its parent LLM span context.
func (r *TraceRegistry) RegisterToolCallMapping(toolCallID string, adkTraceID trace.TraceID, veadkParentSC trace.SpanContext) {
	if toolCallID == "" || !veadkParentSC.IsValid() {
		return
	}
	info := r.getOrCreateToolCallInfo(toolCallID)
	info.mu.Lock()
	info.parentSC = veadkParentSC
	info.mu.Unlock()

	if adkTraceID.IsValid() {
		res := r.getOrCreateTraceInfos(adkTraceID)
		r.resourcesMu.Lock()
		res.toolCallIDs = append(res.toolCallIDs, toolCallID)
		r.resourcesMu.Unlock()
	}
}

// RegisterTraceMapping records a mapping from an internal adk TraceID to a veadk TraceID.
func (r *TraceRegistry) RegisterTraceMapping(adkTraceID trace.TraceID, veadkTraceID trace.TraceID) {
	if !adkTraceID.IsValid() || !veadkTraceID.IsValid() {
		return
	}
	res := r.getOrCreateTraceInfos(adkTraceID)
	r.resourcesMu.Lock()
	res.veadkTraceID = veadkTraceID
	r.resourcesMu.Unlock()
}

// GetVeadkSpanContext finds the veadk replacement for an adk parent span ID.
func (r *TraceRegistry) GetVeadkSpanContext(adkSpanID trace.SpanID) (trace.SpanContext, bool) {
	if val, ok := r.adkSpanMap.Load(adkSpanID); ok {
		return val.(trace.SpanContext), true
	}
	return trace.SpanContext{}, false
}

// GetVeadkParentContextByToolCallID finds the veadk parent for a tool span by its logical ToolCallID.
func (r *TraceRegistry) GetVeadkParentContextByToolCallID(toolCallID string) (trace.SpanContext, bool) {
	if toolCallID == "" {
		return trace.SpanContext{}, false
	}
	if val, ok := r.toolCallMap.Load(toolCallID); ok {
		info := val.(*toolCallInfo)
		info.mu.RLock()
		defer info.mu.RUnlock()
		if info.parentSC.IsValid() {
			return info.parentSC, true
		}
	}
	return trace.SpanContext{}, false
}

// GetVeadkTraceID finds the veadk TraceID for an internal TraceID.
func (r *TraceRegistry) GetVeadkTraceID(adkTraceID trace.TraceID) (trace.TraceID, bool) {
	r.resourcesMu.RLock()
	defer r.resourcesMu.RUnlock()

	if res, ok := r.adkTraceToVeadkTraceMap[adkTraceID]; ok {
		return res.veadkTraceID, res.veadkTraceID.IsValid()
	}
	return trace.TraceID{}, false
}

// UnregisterRunMapping removes run-related mappings.
func (r *TraceRegistry) UnregisterRunMapping(adkSpanID trace.SpanID, veadkSpanID trace.SpanID) {
	r.adkSpanMap.Delete(adkSpanID)
	r.activeInvocationSpans.Delete(veadkSpanID)
}

// ScheduleCleanup schedules cleanup of all mappings related to an internal TraceID.
// This is typically called when the trace is considered complete.
func (r *TraceRegistry) ScheduleCleanup(adkTraceID trace.TraceID, internalRunID trace.SpanID, veadkSpanID trace.SpanID) {
	time.AfterFunc(5*time.Minute, func() {
		r.UnregisterRunMapping(internalRunID, veadkSpanID)

		r.resourcesMu.Lock()
		defer r.resourcesMu.Unlock()

		if res, ok := r.adkTraceToVeadkTraceMap[adkTraceID]; ok {
			// Clean up all spans (merged map makes this simpler)
			for _, sid := range res.spanIDs {
				r.adkSpanMap.Delete(sid)
			}
			// Clean up tool calls
			for _, tcid := range res.toolCallIDs {
				r.toolCallMap.Delete(tcid)
			}
			// Clean up trace map
			delete(r.adkTraceToVeadkTraceMap, adkTraceID)
		}
	})
}

// EndAllInvocationSpans ends all currently active invocation spans.
func (r *TraceRegistry) EndAllInvocationSpans() {
	r.activeInvocationSpans.Range(func(key, value any) bool {
		if span, ok := value.(trace.Span); ok {
			if span.IsRecording() {
				span.End()
			}
		}
		r.activeInvocationSpans.Delete(key)
		return true
	})
}
