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
	"time"

	"google.golang.org/adk/session"
)

const (
	keyMetadata        = "veadk.observability.metadata"
	keyStreamingOutput = "veadk.observability.streaming_output"
	keyStreamingSpan   = "veadk.observability.streaming_span"
	keyAgentSpan       = "veadk.observability.agent_span"
	keyAgentCtx        = "veadk.observability.agent_ctx"
	keyInvocationSpan  = "veadk.observability.invocation_span"
	keyInvocationCtx   = "veadk.observability.invocation_ctx"
	keyModelStartTime  = "veadk.observability.model_start_time"
	keyToolStartTime   = "veadk.observability.tool_start_time"
)

// spanMetadata groups various observational data points in a single structure
// to keep the ADK State clean.
type spanMetadata struct {
	StartTime           time.Time // Start of root invocation
	FirstTokenTime      time.Time // Start of first chunk in a stream
	PromptTokens        int64     // Total session prompt tokens
	CandidateTokens     int64     // Total session output tokens
	TotalTokens         int64     // Total session tokens
	PrevPromptTokens    int64     // Total session prompt tokens before current call
	PrevCandidateTokens int64     // Total session output tokens before current call
	PrevTotalTokens     int64     // Total session tokens before current call
	ModelName           string    // Last model used
}

func (p *observabilityPlugin) getSpanMetadata(state session.State) *spanMetadata {
	val, _ := state.Get(keyMetadata)
	if meta, ok := val.(*spanMetadata); ok {
		return meta
	}
	return &spanMetadata{}
}

func (p *observabilityPlugin) storeSpanMetadata(state session.State, meta *spanMetadata) {
	_ = state.Set(keyMetadata, meta)
}
