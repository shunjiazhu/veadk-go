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

//
// https://volcengine.github.io/veadk-python/observation/span-attributes/
//

const (
	InstrumentationName = "github.com/volcengine/veadk-go"
	DefaultVersion      = "<unknown>" // Aligned with Python ADK
)

// Span names
const (
	SpanInvocation  = "invocation"
	SpanInvokeAgent = "invoke_agent"
	SpanCallLLM     = "call_llm"
	SpanExecuteTool = "execute_tool"
)

// Span attributes
const (
	// General span attributes
	SpanAttrGenAISystemKey        = "gen_ai.system"
	SpanAttrGenAISystemVersionKey = "gen_ai.system.version"
	SpanAttrGenAIAgentNameKey     = "gen_ai.agent.name"
	SpanAttrGenAIAppNameKey       = "gen_ai.app.name"
	SpanAttrGenAIUserIdKey        = "gen_ai.user.id"
	SpanAttrGenAISessionIdKey     = "gen_ai.session.id"
	SpanAttrGenAIInvocationIdKey  = "gen_ai.invocation.id"
	SpanAttrGenAISpanKindKey      = "gen_ai.span.kind"

	// LLM span attributes
	SpanAttrGenAIRequestModelKey                  = "gen_ai.request.model"
	SpanAttrGenAIRequestTypeKey                   = "gen_ai.request.type"
	SpanAttrGenAIRequestMaxTokensKey              = "gen_ai.request.max_tokens"
	SpanAttrGenAIRequestTemperatureKey            = "gen_ai.request.temperature"
	SpanAttrGenAIRequestTopPKey                   = "gen_ai.request.top_p"
	SpanAttrGenAIRequestFunctionsKey              = "gen_ai.request.functions"
	SpanAttrGenAIResponseModelKey                 = "gen_ai.response.model"
	SpanAttrGenAIResponseIdKey                    = "gen_ai.response.id"
	SpanAttrGenAIResponseStopReasonKey            = "gen_ai.response.stop_reason"
	SpanAttrGenAIResponseFinishReasonKey          = "gen_ai.response.finish_reason"
	SpanAttrGenAIResponseFinishReasonsKey         = "gen_ai.response.finish_reasons"
	SpanAttrGenAIIsStreamingKey                   = "gen_ai.is_streaming"
	SpanAttrGenAIPromptKey                        = "gen_ai.prompt"
	SpanAttrGenAICompletionKey                    = "gen_ai.completion"
	SpanAttrGenAIUsageInputTokensKey              = "gen_ai.usage.input_tokens"
	SpanAttrGenAIUsageOutputTokensKey             = "gen_ai.usage.output_tokens"
	SpanAttrGenAIUsageTotalTokensKey              = "gen_ai.usage.total_tokens"
	SpanAttrGenAIUsageCacheCreationInputTokensKey = "gen_ai.usage.cache_creation_input_tokens"
	SpanAttrGenAIUsageCacheReadInputTokensKey     = "gen_ai.usage.cache_read_input_tokens"
	SpanAttrGenAIMessagesKey                      = "gen_ai.messages"
	SpanAttrGenAIChoiceKey                        = "gen_ai.choice"
	SpanAttrGenAIResponsePromptTokenCountKey      = "gen_ai.response.prompt_token_count"
	SpanAttrGenAIResponseCandidatesTokenCountKey  = "gen_ai.response.candidates_token_count"

	// Tool span attributes
	SpanAttrGenAIOperationNameKey = "gen_ai.operation.name"
	SpanAttrGenAIToolNameKey      = "gen_ai.tool.name"
	SpanAttrGenAIToolInputKey     = "gen_ai.tool.input"
	SpanAttrGenAIToolOutputKey    = "gen_ai.tool.output"

	// Platform specific span attributes
	SpanAttrAgentNameKey        = "agent_name"    // Alias of 'gen_ai.agent.name' for CozeLoop platform
	SpanAttrAgentNameDotKey     = "agent.name"    // Alias of 'gen_ai.agent.name' for TLS platform
	SpanAttrAppNameUnderlineKey = "app_name"      // Alias of gen_ai.app.name for CozeLoop platform
	SpanAttrAppNameDotKey       = "app.name"      // Alias of gen_ai.app.name for TLS platform
	SpanAttrUserIdDotKey        = "user.id"       // Alias of gen_ai.user.id for CozeLoop/TLS platforms
	SpanAttrSessionIdDotKey     = "session.id"    // Alias of gen_ai.session.id for CozeLoop/TLS platforms
	SpanAttrInvocationIdDotKey  = "invocation.id" // Alias of gen_ai.invocation.id for CozeLoop platform
	SpanAttrCozeloopInputKey    = "cozeloop.input"
	SpanAttrCozeloopOutputKey   = "cozeloop.output"
	SpanAttrGenAIInputKey       = "gen_ai.input"
	SpanAttrGenAIOutputKey      = "gen_ai.output"
	SpanAttrGenAIInputValueKey  = "input.value"
	SpanAttrGenAIOutputValueKey = "output.value"
)

// Metric names
const (
	// Standard Gen AI Metrics
	MetricNameLLMChatCount                   = "gen_ai.chat.count"
	MetricNameLLMTokenUsage                  = "gen_ai.client.token.usage"
	MetricNameLLMOperationDuration           = "gen_ai.client.operation.duration"
	MetricNameLLMCompletionsExceptions       = "gen_ai.chat_completions.exceptions"
	MetricNameLLMStreamingTimeToFirstToken   = "gen_ai.chat_completions.streaming_time_to_first_token"
	MetricNameLLMStreamingTimeToGenerate     = "gen_ai.chat_completions.streaming_time_to_generate"
	MetricNameLLMStreamingTimePerOutputToken = "gen_ai.chat_completions.streaming_time_per_output_token"

	// APMPlus Custom Metrics
	MetricNameAPMPlusSpanLatency    = "apmplus_span_latency"
	MetricNameAPMPlusToolTokenUsage = "apmplus_tool_token_usage"
)

// Metric attributes
const (
	// Standard Gen AI Metric Attributes
	MetricAttrGenAIRequestModelKey = "gen_ai.request.model"

	// APMPlus Custom Metric Attributes
	MetricAttrGenAIRequestModelKeyForApmplus = "gen_ai.request.model"
)

// General constants
const (
	// Instrumentation
	InstrumentationKey = "openinference.instrumentation.veadk"

	// Platform specific
	CozeloopReportSourceKey = "cozeloop.report.source" // Fixed value: veadk
	CozeloopCallTypeKey     = "cozeloop.call_type"     // CozeLoop call type

	// Environment Variable Keys for Zero-Config Attributes
	EnvModelProvider = "VEADK_MODEL_PROVIDER"
	EnvUserId        = "VEADK_USER_ID"
	EnvSessionId     = "VEADK_SESSION_ID"
	EnvAppName       = "VEADK_APP_NAME"
	EnvAgentName     = "VEADK_AGENT_NAME"
	EnvCallType      = "VEADK_CALL_TYPE"

	// Default and fallback values
	DefaultCozeLoopCallType     = ""      // fixed, matches Python's None behavior
	DefaultCozeLoopReportSource = "veadk" // fixed
	FallbackAgentName           = "<unknown_agent_name>"
	FallbackAppName             = "<unknown_app_name>"
	FallbackUserID              = "<unknown_user_id>"
	FallbackSessionID           = "<unknown_session_id>"
	FallbackInvocationID        = "<unknown_invocation_id>"
	FallbackModelProvider       = "<unknown_model_provider>"

	// Span Kind values (GenAI semantic conventions)
	SpanKindWorkflow = "workflow"
	SpanKindLLM      = "llm"
	SpanKindTool     = "tool"
)

// Context keys for storing runtime values
type contextKey string

const (
	ContextKeySessionId     contextKey = "veadk.session_id"
	ContextKeyUserId        contextKey = "veadk.user_id"
	ContextKeyAppName       contextKey = "veadk.app_name"
	ContextKeyAgentName     contextKey = "veadk.agent_name"
	ContextKeyCallType      contextKey = "veadk.call_type"
	ContextKeyModelProvider contextKey = "veadk.model_provider"
	ContextKeyInvocationId  contextKey = "veadk.invocation_id"
)
