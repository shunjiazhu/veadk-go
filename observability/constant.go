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

// Span names
const (
	SpanInvocation  = "invocation"
	SpanInvokeAgent = "invoke_agent"
	SpanCallLLM     = "call_llm"
	SpanExecuteTool = "execute_tool"
)

// Metric names
const (
	MetricNameTokenUsage        = "gen_ai.client.token.usage"
	MetricNameOperationDuration = "gen_ai.client.operation.duration"
	MetricNameFirstTokenLatency = "gen_ai.client.token.first_token_latency"
)

// General attributes
const (
	AttrGenAISystem        = "gen_ai.system"
	AttrGenAISystemVersion = "gen_ai.system.version"

	AttrGenAIAgentName = "gen_ai.agent.name"
	AttrAgentName      = "agent_name" // Alias for backward compatibility/platform

	AttrAppName   = "gen_ai.app.name"
	AttrUserId    = "gen_ai.user.id"
	AttrSessionId = "gen_ai.session.id"

	AttrCozeloopReportSource = "cozeloop.report.source"
	ValGenAISystem           = "veadk"
	ValCozeloopReportSource  = "veadk"
)

// LLM attributes
const (
	AttrGenAIRequestModel       = "gen_ai.request.model"
	AttrGenAIRequestType        = "gen_ai.request.type"
	AttrGenAIRequestMaxTokens   = "gen_ai.request.max_tokens"
	AttrGenAIRequestTemperature = "gen_ai.request.temperature"
	AttrGenAIRequestTopP        = "gen_ai.request.top_p"
	AttrGenAIRequestFunctions   = "gen_ai.request.functions"

	AttrGenAIResponseModel         = "gen_ai.response.model"
	AttrGenAIResponseId            = "gen_ai.response.id"
	AttrGenAIResponseFinishReasons = "gen_ai.response.finish_reasons"

	AttrGenAIIsStreaming = "gen_ai.is_streaming"

	AttrGenAIUsageInputTokens              = "gen_ai.usage.input_tokens"
	AttrGenAIUsageOutputTokens             = "gen_ai.usage.output_tokens"
	AttrGenAIUsageTotalTokens              = "gen_ai.usage.total_tokens"
	AttrGenAIUsageCacheCreationInputTokens = "gen_ai.usage.cache_creation_input_tokens"
	AttrGenAIUsageCacheReadInputTokens     = "gen_ai.usage.cache_read_input_tokens"

	AttrGenAIMessages = "gen_ai.messages"
	AttrGenAIChoice   = "gen_ai.choice"

	AttrGenAIInputValue  = "input.value"
	AttrGenAIOutputValue = "output.value"
)

// Tool attributes
const (
	AttrGenAIOperationName = "gen_ai.operation.name"
	AttrGenAIToolName      = "gen_ai.tool.name"
	AttrGenAIToolInput     = "gen_ai.tool.input"
	AttrGenAIToolOutput    = "gen_ai.tool.output"
	AttrGenAISpanKind      = "gen_ai.span.kind"

	// Platform specific
	AttrCozeloopInput  = "cozeloop.input"
	AttrCozeloopOutput = "cozeloop.output"
	AttrGenAIInput     = "gen_ai.input"
	AttrGenAIOutput    = "gen_ai.output"
)

// Context keys
type contextKey string

const (
	ContextKeySessionId contextKey = "veadk_session_id"
	ContextKeyUserId    contextKey = "veadk_user_id"
	ContextKeyAppName   contextKey = "veadk_app_name"
)
