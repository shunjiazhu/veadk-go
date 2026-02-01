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
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"

	"github.com/volcengine/veadk-go/configs"
	"github.com/volcengine/veadk-go/log"
	"github.com/volcengine/veadk-go/observability/exporter"
)

// NewPlugin creates a new observability plugin for ADK.
// It returns a *plugin.Plugin that can be registered in launcher.Config or agent.Config.
func NewPlugin(opts ...Option) *plugin.Plugin {
	// Ensure observability system is initialized.
	// This will use default configuration or environment variables.
	observabilityConfig := configs.GetGlobalConfig().Observability.Clone()
	// Apply options
	for _, opt := range opts {
		opt(observabilityConfig)
	}

	p := &adkObservabilityPlugin{
		config: observabilityConfig,
	}

	err := Init(context.Background(), observabilityConfig)
	if err != nil {
		log.Error("Init observability failed", "error", err)
		return nil
	}

	p.tracer = otel.Tracer(InstrumentationName)

	pluginInstance, _ := plugin.New(plugin.Config{
		Name:                "veadk-observability",
		BeforeRunCallback:   p.BeforeRun,
		AfterRunCallback:    p.AfterRun,
		BeforeModelCallback: p.BeforeModel,
		AfterModelCallback:  p.AfterModel,
		BeforeToolCallback:  p.BeforeTool,
		AfterToolCallback:   p.AfterTool,
		BeforeAgentCallback: p.BeforeAgent,
		AfterAgentCallback:  p.AfterAgent,
	})
	return pluginInstance
}

// Option defines a functional option for the ADKObservabilityPlugin.
type Option func(config *configs.ObservabilityConfig)

// WithEnableMetrics creates an Option to manually control metrics recording.
func WithEnableMetrics(enable bool) Option {
	return func(cfg *configs.ObservabilityConfig) {
		enableVal := enable
		cfg.OpenTelemetry.EnableMetrics = &enableVal
	}
}

type adkObservabilityPlugin struct {
	config *configs.ObservabilityConfig

	tracer trace.Tracer // global tracer
}

func (p *adkObservabilityPlugin) isMetricsEnabled() bool {
	if p.config == nil || p.config.OpenTelemetry == nil {
		return false
	}

	if p.config.OpenTelemetry.EnableMetrics != nil {
		return *p.config.OpenTelemetry.EnableMetrics
	}

	return false
}

// BeforeRun is called before an agent run starts.
func (p *adkObservabilityPlugin) BeforeRun(ctx agent.InvocationContext) (*genai.Content, error) {
	// 1. Check if we're already inside an invocation span to avoid duplicates
	existingSpan := trace.SpanFromContext(context.Context(ctx))
	if existingSpan.SpanContext().IsValid() && existingSpan.IsRecording() {
		// If we are already inside an invocation span, we can reuse it
		// but typically we want the plugin to be self-contained.
	}

	// 2. Start the 'invocation' span
	// Align with Python: name is "invocation"
	// Use SpanKindServer for the root invocation span
	newCtx, span := p.tracer.Start(context.Context(ctx), SpanInvocation, trace.WithSpanKind(trace.SpanKindServer))

	// 3. Store in state for AfterRun and children
	_ = ctx.Session().State().Set(stateKeyInvocationSpan, span)
	_ = ctx.Session().State().Set(stateKeyInvocationCtx, newCtx)

	// 4. Register this span as the root for this TraceID
	sc := span.SpanContext()
	if sc.IsValid() {
		exporter.RegisterInvocationSpan(sc.TraceID(), span)
	}

	// 5. Set attributes
	setCommonAttributes(newCtx, span)
	setWorkflowAttributes(span)

	// Record start time for metrics
	meta := &spanMetadata{
		StartTime: time.Now(),
	}
	p.storeSpanMetadata(ctx.Session().State(), meta)

	// Capture input from UserContent
	if userContent := ctx.UserContent(); userContent != nil {
		if jsonIn, err := json.Marshal(userContent); err == nil {
			span.SetAttributes(attribute.String(AttrGenAIInputValue, string(jsonIn)))
		}
	}

	return nil, nil
}

// AfterRun is called after an agent run ends.
func (p *adkObservabilityPlugin) AfterRun(ctx agent.InvocationContext) {
	// 1. End the span
	if s, _ := ctx.Session().State().Get(stateKeyInvocationSpan); s != nil {
		span := s.(trace.Span)
		if span.IsRecording() {
			// Clean up from global map
			sc := span.SpanContext()
			if sc.IsValid() {
				exporter.UnregisterInvocationSpan(sc.TraceID())
				exporter.UnregisterAgentSpanContext(sc.TraceID())
				exporter.UnregisterLLMSpanContext(sc.TraceID())
			}

			// Capture final output if available
			if cached, _ := ctx.Session().State().Get(stateKeyStreamingOutput); cached != nil {
				if jsonOut, err := json.Marshal(cached); err == nil {
					span.SetAttributes(attribute.String(AttrGenAIOutputValue, string(jsonOut)))
				}
			}
			// Capture accumulated token usage for the root invocation span
			meta := p.getSpanMetadata(ctx.Session().State())

			if meta.PromptTokens > 0 {
				span.SetAttributes(attribute.Int64(AttrGenAIUsageInputTokens, meta.PromptTokens))
			}
			if meta.CandidateTokens > 0 {
				span.SetAttributes(attribute.Int64(AttrGenAIUsageOutputTokens, meta.CandidateTokens))
			}
			if meta.TotalTokens > 0 {
				span.SetAttributes(attribute.Int64(AttrGenAIUsageTotalTokens, meta.TotalTokens))
			}

			// Record final metrics for invocation
			if !meta.StartTime.IsZero() {
				elapsed := time.Since(meta.StartTime).Seconds()
				metricAttrs := []attribute.KeyValue{
					attribute.String("gen_ai_operation_name", "chain"),
					attribute.String("gen_ai_operation_type", "workflow"),
					attribute.String("gen_ai.system", "veadk"),
				}
				if p.isMetricsEnabled() {
					RecordOperationDuration(context.Background(), elapsed, metricAttrs...)
					RecordAPMPlusSpanLatency(context.Background(), elapsed, metricAttrs...)
				}
			}

			span.End()
		}
	}
}

// BeforeModel is called before the LLM is called.
func (p *adkObservabilityPlugin) BeforeModel(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	// 1. Get the parent context from state to maintain hierarchy
	parentCtx := context.Context(ctx)
	if actx, _ := ctx.State().Get(stateKeyAgentCtx); actx != nil {
		parentCtx = actx.(context.Context)
	} else if ictx, _ := ctx.State().Get(stateKeyInvocationCtx); ictx != nil {
		parentCtx = ictx.(context.Context)
	}

	// 2. Start our OWN span to cover the full duration of the call (including streaming).
	// ADK's "call_llm" span will be closed prematurely by the framework on the first chunk.
	// Align with Python: name is "call_llm"
	newCtx, span := p.tracer.Start(parentCtx, SpanCallLLM)
	_ = ctx.State().Set(stateKeyStreamingSpan, span)

	// Register LLM span context for re-parenting subsequent tools
	sc := span.SpanContext()
	if sc.IsValid() {
		exporter.RegisterLLMSpanContext(sc.TraceID(), sc)
	}

	// Group metadata in a single structure for state storage
	meta := p.getSpanMetadata(ctx.State())
	meta.StartTime = time.Now()
	meta.PrevPromptTokens = meta.PromptTokens
	meta.PrevCandidateTokens = meta.CandidateTokens
	meta.PrevTotalTokens = meta.TotalTokens
	meta.ModelName = req.Model
	p.storeSpanMetadata(ctx.State(), meta)

	// Link back to the ADK internal span if it's there.
	// This records the ID of the span started by the ADK framework, which we
	// often bypass to maintain a cleaner hierarchy in our manual spans.
	adkSpan := trace.SpanFromContext(context.Context(ctx))
	if adkSpan.SpanContext().IsValid() {
		span.SetAttributes(attribute.String("adk.internal_span_id", adkSpan.SpanContext().SpanID().String()))
	}

	setCommonAttributes(newCtx, span)
	// Set GenAI standard span attributes
	setLLMAttributes(span)

	// Record request attributes
	attrs := []attribute.KeyValue{
		attribute.String(AttrGenAIRequestModel, req.Model),
		attribute.String(AttrGenAIRequestType, "chat"), // Default to chat
		attribute.String(AttrGenAISystem, GetModelProvider(context.Context(ctx))),
	}

	if req.Config != nil {
		if req.Config.Temperature != nil {
			attrs = append(attrs, attribute.Float64(AttrGenAIRequestTemperature, float64(*req.Config.Temperature)))
		}
		if req.Config.TopP != nil {
			attrs = append(attrs, attribute.Float64(AttrGenAIRequestTopP, float64(*req.Config.TopP)))
		}
		if req.Config.MaxOutputTokens > 0 {
			attrs = append(attrs, attribute.Int64(AttrGenAIRequestMaxTokens, int64(req.Config.MaxOutputTokens)))
		}
		if len(req.Config.Tools) > 0 {
			for i, tool := range req.Config.Tools {
				if tool.FunctionDeclarations != nil {
					for j, fn := range tool.FunctionDeclarations {
						prefix := fmt.Sprintf("gen_ai.request.functions.%d.", i+j) // Simplified indexing
						attrs = append(attrs, attribute.String(prefix+"name", fn.Name))
						attrs = append(attrs, attribute.String(prefix+"description", fn.Description))
						if fn.Parameters != nil {
							paramsJSON, _ := json.Marshal(fn.Parameters)
							attrs = append(attrs, attribute.String(prefix+"parameters", string(paramsJSON)))
						}
					}
				}
			}
		}
	}

	// Capture messages in GenAI format for the span
	messages := p.extractMessages(req)
	messagesJSON, err := json.Marshal(messages)
	if err == nil {
		attrs = append(attrs, attribute.String(AttrGenAIMessages, string(messagesJSON)))
	}

	// Flatten messages for gen_ai.prompt.[n] attributes (alignment with python)
	attrs = append(attrs, p.flattenPrompt(messages)...)

	// Add input.value (standard for some collectors)
	attrs = append(attrs, attribute.String(AttrGenAIInput, string(messagesJSON)))
	attrs = append(attrs, attribute.String(AttrGenAIInputValue, string(messagesJSON)))

	span.SetAttributes(attrs...)

	// Add gen_ai.user.message event (aligned with Python)
	p.addUserMessageEvents(span, ctx, req)

	// Add gen_ai.content.prompt event (OTEL GenAI convention)
	span.AddEvent(EventGenAIContentPrompt, trace.WithAttributes(
		attribute.String(AttrGenAIPrompt, string(messagesJSON)),
		attribute.String(AttrGenAIInput, string(messagesJSON)),
	))

	return nil, nil
}

// AfterModel is called after the LLM returns.
func (p *adkObservabilityPlugin) AfterModel(ctx agent.CallbackContext, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
	// 1. Get our managed span from state
	s, _ := ctx.State().Get(stateKeyStreamingSpan)
	if s == nil {
		return nil, nil
	}
	span := s.(trace.Span)

	// 2. Wrap the cleanup to ensure span is always ended on error or final chunk.
	// ADK calls AfterModel for EVERY chunk in a stream.
	// resp.Partial is true for intermediate chunks, false for the final one.
	defer func() {
		if err != nil || (resp != nil && !resp.Partial) {
			if span.IsRecording() {
				span.End()
			}
		}
	}()

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, nil
	}

	if resp == nil {
		return nil, nil
	}

	if !span.IsRecording() {
		// Even if not recording, we should still accumulate content for metrics/logs
	}

	// Record responding model
	meta := p.getSpanMetadata(ctx.State())
	// Try to get confirmation from response metadata first (passed from sdk)
	var finalModelName string
	if resp.CustomMetadata != nil {
		if m, ok := resp.CustomMetadata["response_model"].(string); ok {
			finalModelName = m
		}
	}
	// Fallback to request model name if not present in response
	if finalModelName == "" {
		finalModelName = meta.ModelName
	}
	if finalModelName != "" {
		span.SetAttributes(attribute.String(AttrGenAIResponseModel, finalModelName))
	}

	if resp.UsageMetadata != nil {
		p.handleUsage(ctx, span, resp, resp.Partial, finalModelName)
	}

	if resp.FinishReason != "" {
		span.SetAttributes(attribute.String(AttrGenAIResponseFinishReason, string(resp.FinishReason)))
	}

	// Record response content
	var currentAcc *genai.Content
	cached, _ := ctx.State().Get(stateKeyStreamingOutput)
	if cached != nil {
		currentAcc = cached.(*genai.Content)
	}

	// ---------------------------------------------------------
	// Metrics: Time to First Token (Streaming Only)
	// ---------------------------------------------------------
	if resp.Partial && currentAcc == nil && resp.Content != nil {
		// This is the very first chunk
		if !meta.StartTime.IsZero() {
			meta.FirstTokenTime = time.Now()
			p.storeSpanMetadata(ctx.State(), meta)

			if p.isMetricsEnabled() {
				// Record streaming time to first token
				latency := time.Since(meta.StartTime).Seconds()
				metricAttrs := []attribute.KeyValue{
					attribute.String(AttrGenAISystem, "veadk"),
					attribute.String("gen_ai_response_model", finalModelName),
					attribute.String("gen_ai_operation_name", "chat"),
					attribute.String("gen_ai_operation_type", "llm"),
				}
				RecordStreamingTimeToFirstToken(context.Context(ctx), latency, metricAttrs...)
			}
		}
	}

	if resp.Content != nil {
		if currentAcc == nil {
			currentAcc = &genai.Content{Role: resp.Content.Role}
			if currentAcc.Role == "" {
				currentAcc.Role = "model"
			}
		}

		// If this is the final response, our implementation (like OpenAI) often sends the full content.
		// We clear our previous accumulation to avoid duplication in the span attributes.
		// We only do this if the final response actually contains content.
		if !resp.Partial && resp.Content != nil && len(resp.Content.Parts) > 0 {
			currentAcc.Parts = nil
		}

		// Accumulate parts with merging of adjacent text
		for _, part := range resp.Content.Parts {
			// If it's a text part, try to merge with the last part if that was also text
			if part.Text != "" && len(currentAcc.Parts) > 0 {
				lastPart := currentAcc.Parts[len(currentAcc.Parts)-1]
				if lastPart.Text != "" && lastPart.FunctionCall == nil && lastPart.FunctionResponse == nil && lastPart.InlineData == nil {
					lastPart.Text += part.Text
					continue
				}
			}

			// Otherwise append as a new part
			newPart := &genai.Part{}
			*newPart = *part
			currentAcc.Parts = append(currentAcc.Parts, newPart)
		}
		_ = ctx.State().Set(stateKeyStreamingOutput, currentAcc)
	}

	// For streaming, we update the span attributes with what we have so far
	var fullText string
	if currentAcc != nil {
		// Set output.value to the cumulative text (parity with python)
		var textParts []string
		for _, p := range currentAcc.Parts {
			if p.Text != "" {
				textParts = append(textParts, p.Text)
			}
		}
		fullText = strings.Join(textParts, "")
		span.SetAttributes(attribute.String(AttrGenAIOutput, fullText))

		// Add output.value for full JSON representation
		if contentJSON, err := json.Marshal(currentAcc); err == nil {
			span.SetAttributes(attribute.String("output.value", string(contentJSON)))
		}

		// Also set the structured GenAI attributes
		span.SetAttributes(p.flattenCompletion(currentAcc)...)
	}

	// Metrics: Time to Generate (Streaming Only) & Time Per Output Token
	if !resp.Partial && currentAcc != nil {
		if !meta.StartTime.IsZero() {
			_ = time.Since(meta.StartTime).Seconds()

			if p.isMetricsEnabled() {
				// TODO: Alignment with Python - Python currently has these as TODOs
				// RecordStreamingTimeToGenerate(context.Context(ctx), totalDuration, metricAttrs...)
			}

			// Time Per Output Token
			// Only valid if we have output tokens and we tracked first token time
			if meta.CandidateTokens > 0 {
				generateDuration := time.Since(meta.FirstTokenTime).Seconds()
				metricAttrs := []attribute.KeyValue{
					attribute.String(AttrGenAISystem, "veadk"),
					attribute.String("gen_ai_response_model", finalModelName),
					attribute.String("gen_ai_operation_name", "chat"),
					attribute.String("gen_ai_operation_type", "llm"),
				}
				RecordStreamingTimeToGenerate(context.Context(ctx), generateDuration, metricAttrs...)

				if generateDuration > 0 {
					timePerToken := generateDuration / float64(meta.CandidateTokens)
					RecordStreamingTimePerOutputToken(context.Context(ctx), timePerToken, metricAttrs...)
				}
			}
		}
	}

	// If this is the final chunk, add the completion event
	if !resp.Partial && currentAcc != nil {
		contentJSON, _ := json.Marshal(currentAcc)
		span.AddEvent(EventGenAIContentCompletion, trace.WithAttributes(
			attribute.String(AttrGenAICompletion, string(contentJSON)),
			attribute.String(AttrGenAIOutput, fullText),
		))

		// Add gen_ai.choice event (aligned with Python)
		p.addChoiceEvents(span, currentAcc)
	}

	// If this is the final chunk (or non-streaming response), record final metrics
	if !resp.Partial {

		// Record Operation Duration and Latency
		if !meta.StartTime.IsZero() {
			duration := time.Since(meta.StartTime).Seconds()
			metricAttrs := []attribute.KeyValue{
				attribute.String(AttrGenAISystem, "veadk"),
				attribute.String("gen_ai_response_model", finalModelName),
				attribute.String("gen_ai_operation_name", "chat"),
				attribute.String("gen_ai_operation_type", "llm"),
			}
			if p.isMetricsEnabled() {
				RecordOperationDuration(context.Context(ctx), duration, metricAttrs...)
				RecordAPMPlusSpanLatency(context.Context(ctx), duration, metricAttrs...)
			}

			// Record Exceptions if needed (though usually handled via err check at top)
			// But if we want to record "success" implicit via lack of exception metric, that's fine.
			// span_enrich recorded exception if status code was Error.
			// We handled err at top of function.
		}
	}

	return nil, nil
}

func (p *adkObservabilityPlugin) handleUsage(ctx agent.CallbackContext, span trace.Span, resp *model.LLMResponse, isStream bool, modelName string) {
	meta := p.getSpanMetadata(ctx.State())

	// 1. Get current call usage
	currentPrompt := int64(resp.UsageMetadata.PromptTokenCount)
	currentCandidate := int64(resp.UsageMetadata.CandidatesTokenCount)
	currentTotal := int64(resp.UsageMetadata.TotalTokenCount)

	if currentTotal == 0 && (currentPrompt > 0 || currentCandidate > 0) {
		currentTotal = currentPrompt + currentCandidate
	}

	// 2. New session total = previous calls total + current call's (latest) usage
	// (Note: in streaming, currentCall usage is cumulative for this call)
	meta.PromptTokens = meta.PrevPromptTokens + currentPrompt
	meta.CandidateTokens = meta.PrevCandidateTokens + currentCandidate
	meta.TotalTokens = meta.PrevTotalTokens + currentTotal

	// 3. Update session-wide totals
	p.storeSpanMetadata(ctx.State(), meta)

	// 4. Set attributes on the current LLM span (only current call's usage)
	attrs := []attribute.KeyValue{}
	if currentPrompt > 0 {
		attrs = append(attrs, attribute.Int64(AttrGenAIUsageInputTokens, currentPrompt))
		attrs = append(attrs, attribute.Int64(AttrGenAIResponsePromptTokenCount, currentPrompt))
	}
	if currentCandidate > 0 {
		attrs = append(attrs, attribute.Int64(AttrGenAIUsageOutputTokens, currentCandidate))
		attrs = append(attrs, attribute.Int64(AttrGenAIResponseCandidatesTokenCount, currentCandidate))
	}
	if currentTotal > 0 {
		attrs = append(attrs, attribute.Int64(AttrGenAIUsageTotalTokens, currentTotal))
	}

	if resp.UsageMetadata != nil {
		if resp.UsageMetadata.CachedContentTokenCount > 0 {
			attrs = append(attrs, attribute.Int64(AttrGenAIUsageCacheReadInputTokens, int64(resp.UsageMetadata.CachedContentTokenCount)))
		}
		// Always set cache creation to 0 if not provided, for parity with python
		attrs = append(attrs, attribute.Int64(AttrGenAIUsageCacheCreationInputTokens, 0))
	}

	span.SetAttributes(attrs...)

	// Record metrics directly from the plugin logic
	if p.isMetricsEnabled() && (currentPrompt > 0 || currentCandidate > 0) {
		metricAttrs := []attribute.KeyValue{
			attribute.String(AttrGenAISystem, "veadk"),
			attribute.String("gen_ai_response_model", modelName),
			attribute.String("gen_ai_operation_name", "chat"),
			attribute.String("gen_ai_operation_type", "llm"),
		}
		RecordTokenUsage(context.Context(ctx), currentPrompt, currentCandidate, metricAttrs...)
		RecordChatCount(context.Context(ctx), 1, metricAttrs...)
	}
}

func (p *adkObservabilityPlugin) addUserMessageEvents(span trace.Span, ctx agent.CallbackContext, req *model.LLMRequest) {
	// Add gen_ai.user.message event
	// Use agent/app/user info if available from context
	// Since agent.CallbackContext might not give easy access to all metadata,
	// we will try to mimic Python behavior which uses stored context.
	// For now, we will just dump what we have in request if possible.

	for _, content := range req.Contents {
		if content.Role != "user" {
			continue
		}

		for i, part := range content.Parts {
			attrs := []attribute.KeyValue{}
			if part.Text != "" {
				attrs = append(attrs, attribute.String("parts."+strconv.Itoa(i)+".type", "text"))
				attrs = append(attrs, attribute.String("parts."+strconv.Itoa(i)+".content", part.Text))
			}
			// TODO: Handle other part types if needed for full alignment

			if len(attrs) > 0 {
				span.AddEvent("gen_ai.user.message", trace.WithAttributes(attrs...))
			}
		}
	}
}

func (p *adkObservabilityPlugin) addChoiceEvents(span trace.Span, content *genai.Content) {
	for i, part := range content.Parts {
		attrs := []attribute.KeyValue{}
		if part.Text != "" {
			attrs = append(attrs, attribute.String("message.parts."+strconv.Itoa(i)+".type", "text"))
			attrs = append(attrs, attribute.String("message.parts."+strconv.Itoa(i)+".text", part.Text))
		}
		if len(attrs) > 0 {
			span.AddEvent("gen_ai.choice", trace.WithAttributes(attrs...))
		}
	}
}

// extractMessages converts ADK model.LLMRequest contents into a JSON-compatible message list.
func (p *adkObservabilityPlugin) extractMessages(req *model.LLMRequest) []map[string]any {
	var messages []map[string]any
	for _, content := range req.Contents {
		role := content.Role
		if role == "model" {
			role = "assistant"
		}

		msg := map[string]any{
			"role": role,
		}

		var textParts []string
		var toolCalls []map[string]any
		var toolResponses []map[string]any

		for _, part := range content.Parts {
			if part.Text != "" {
				textParts = append(textParts, part.Text)
			}
			if part.FunctionCall != nil {
				toolCalls = append(toolCalls, map[string]any{
					"id":   part.FunctionCall.ID,
					"type": "function",
					"function": map[string]any{
						"name":      part.FunctionCall.Name,
						"arguments": safeMarshal(part.FunctionCall.Args),
					},
				})
			}
			if part.FunctionResponse != nil {
				toolResponses = append(toolResponses, map[string]any{
					"id":      part.FunctionResponse.ID,
					"name":    part.FunctionResponse.Name,
					"content": safeMarshal(part.FunctionResponse.Response),
				})
			}
		}

		if len(textParts) > 0 {
			msg["content"] = strings.Join(textParts, "")
		}
		if len(toolCalls) > 0 {
			msg["tool_calls"] = toolCalls
		}
		if len(toolResponses) > 0 {
			// In standard GenAI, tool responses are often represented separate messages or differently.
			// Alignment with veadk-python usually means following their structure.
			msg["tool_responses"] = toolResponses
		}

		messages = append(messages, msg)
	}
	return messages
}

func (p *adkObservabilityPlugin) flattenPrompt(messages []map[string]any) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	idx := 0
	for _, msg := range messages {
		// In Python, each piece of content/part increments the index.
		// Since we already merged text parts in extractMessages, we just process each message here.
		// If we wanted exact parity for multi-part messages, we'd need to change extractMessages.
		// For now, this is a good approximation that matches the role/content flat structure.
		prefix := "gen_ai.prompt." + strconv.Itoa(idx)
		if role, ok := msg["role"].(string); ok {
			attrs = append(attrs, attribute.String(prefix+".role", role))
		}
		if content, ok := msg["content"].(string); ok {
			attrs = append(attrs, attribute.String(prefix+".content", content))
		}

		if toolCalls, ok := msg["tool_calls"].([]map[string]any); ok {
			for j, tc := range toolCalls {
				tcPrefix := prefix + ".tool_calls." + strconv.Itoa(j)
				if id, ok := tc["id"].(string); ok {
					attrs = append(attrs, attribute.String(tcPrefix+".id", id))
				}
				if t, ok := tc["type"].(string); ok {
					attrs = append(attrs, attribute.String(tcPrefix+".type", t))
				}
				if fn, ok := tc["function"].(map[string]any); ok {
					if name, ok := fn["name"].(string); ok {
						attrs = append(attrs, attribute.String(tcPrefix+".function.name", name))
					}
					if args, ok := fn["arguments"].(string); ok {
						attrs = append(attrs, attribute.String(tcPrefix+".function.arguments", args))
					}
				}
			}
		}

		if toolResponses, ok := msg["tool_responses"].([]map[string]any); ok {
			for j, tr := range toolResponses {
				trPrefix := prefix + ".tool_responses." + strconv.Itoa(j)
				if id, ok := tr["id"].(string); ok {
					attrs = append(attrs, attribute.String(trPrefix+".id", id))
				}
				if name, ok := tr["name"].(string); ok {
					attrs = append(attrs, attribute.String(trPrefix+".name", name))
				}
				if content, ok := tr["content"].(string); ok {
					attrs = append(attrs, attribute.String(trPrefix+".content", content))
				}
			}
		}
		idx++
	}
	return attrs
}

func (p *adkObservabilityPlugin) flattenCompletion(content *genai.Content) []attribute.KeyValue {
	var attrs []attribute.KeyValue

	role := content.Role
	if role == "model" {
		role = "assistant"
	}

	for idx, part := range content.Parts {
		prefix := "gen_ai.completion." + strconv.Itoa(idx)
		attrs = append(attrs, attribute.String(prefix+".role", role))

		if part.Text != "" {
			attrs = append(attrs, attribute.String(prefix+".content", part.Text))
		}
		if part.FunctionCall != nil {
			tcPrefix := prefix + ".tool_calls.0"
			attrs = append(attrs, attribute.String(tcPrefix+".id", part.FunctionCall.ID))
			attrs = append(attrs, attribute.String(tcPrefix+".type", "function"))
			attrs = append(attrs, attribute.String(tcPrefix+".function.name", part.FunctionCall.Name))
			attrs = append(attrs, attribute.String(tcPrefix+".function.arguments", safeMarshal(part.FunctionCall.Args)))
		}
	}

	return attrs
}

func safeMarshal(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// AfterTool is called after a tool is executed.
func (p *adkObservabilityPlugin) AfterTool(ctx tool.Context, tool tool.Tool, args, result map[string]any, err error) (map[string]any, error) {
	span := trace.SpanFromContext(context.Context(ctx))
	if !span.IsRecording() {
		return nil, nil
	}

	activeCtx := context.Context(ctx)

	// Set GenAI standard span attributes
	setCommonAttributes(activeCtx, span)
	setToolAttributes(span, tool.Name())

	// Enrich standard attributes
	argsJSON, _ := json.Marshal(args)
	resultJSON, _ := json.Marshal(result)

	attrs := []attribute.KeyValue{
		attribute.String(AttrGenAIToolInput, string(argsJSON)),
		attribute.String(AttrGenAIToolOutput, string(resultJSON)),
	}
	span.SetAttributes(attrs...)

	// Metrics
	meta := p.getSpanMetadata(ctx.State())
	if !meta.StartTime.IsZero() {
		duration := time.Since(meta.StartTime).Seconds()
		metricAttrs := []attribute.KeyValue{
			attribute.String("gen_ai_operation_name", tool.Name()),
			attribute.String("gen_ai_operation_type", "tool"),
			attribute.String("gen_ai_system", "veadk"),
		}
		if p.isMetricsEnabled() {
			RecordOperationDuration(context.Context(ctx), duration, metricAttrs...)
			RecordAPMPlusSpanLatency(context.Context(ctx), duration, metricAttrs...)
		}

		// Tool Token Usage (Estimated)
		inputChars := int64(len(string(argsJSON)))
		outputChars := int64(len(string(resultJSON)))

		if p.isMetricsEnabled() {
			if inputChars > 0 {
				RecordAPMPlusToolTokenUsage(context.Context(ctx), inputChars/4, append(metricAttrs, attribute.String("token_type", "input"))...)
			}
			if outputChars > 0 {
				RecordAPMPlusToolTokenUsage(context.Context(ctx), outputChars/4, append(metricAttrs, attribute.String("token_type", "output"))...)
			}
		}
	}

	return nil, nil
}

func (p *adkObservabilityPlugin) BeforeTool(ctx tool.Context, tool tool.Tool, args map[string]any) (map[string]any, error) {
	meta := p.getSpanMetadata(ctx.State())
	meta.StartTime = time.Now()
	p.storeSpanMetadata(ctx.State(), meta)
	return nil, nil
}

// BeforeAgent is called before an agent execution.
func (p *adkObservabilityPlugin) BeforeAgent(ctx agent.CallbackContext) (*genai.Content, error) {
	agentName := ctx.AgentName()
	if agentName == "" {
		agentName = FallbackAgentName
	}

	// 1. Get the parent context from state to maintain hierarchy
	parentCtx := context.Context(ctx)
	if ictx, _ := ctx.State().Get(stateKeyInvocationCtx); ictx != nil {
		parentCtx = ictx.(context.Context)
	}

	// 2. Start the 'invoke_agent' span manually.
	// Since we can't easily wrap the Agent interface due to internal methods,
	// we use the plugin to start our span.
	spanName := SpanInvokeAgent + " " + agentName
	newCtx, span := p.tracer.Start(parentCtx, spanName)

	// 3. Store in state for AfterAgent and children
	_ = ctx.State().Set(stateKeyAgentSpan, span)
	_ = ctx.State().Set(stateKeyAgentCtx, newCtx)

	// 3. Register this span as the current parent for ADK internal spans in this trace.
	// This is the key to fixing hierarchy perfectly.
	sc := span.SpanContext()
	if sc.IsValid() {
		exporter.RegisterAgentSpanContext(sc.TraceID(), sc)
	}

	// 4. Set attributes
	setCommonAttributes(newCtx, span)
	setWorkflowAttributes(span)
	setAgentAttributes(span, agentName)

	return nil, nil
}

// AfterAgent is called after an agent execution.
func (p *adkObservabilityPlugin) AfterAgent(ctx agent.CallbackContext) (*genai.Content, error) {
	// 1. End the span
	if s, _ := ctx.State().Get(stateKeyAgentSpan); s != nil {
		span := s.(trace.Span)
		if span.IsRecording() {
			// Clean up from global map using the actual span's TraceID
			sc := span.SpanContext()
			if sc.IsValid() {
				exporter.UnregisterAgentSpanContext(sc.TraceID())
			}
			span.End()
		}
	}
	return nil, nil
}

func (p *adkObservabilityPlugin) getSpanMetadata(state session.State) *spanMetadata {
	val, _ := state.Get(stateKeyMetadata)
	if meta, ok := val.(*spanMetadata); ok {
		return meta
	}
	return &spanMetadata{}
}

func (p *adkObservabilityPlugin) storeSpanMetadata(state session.State, meta *spanMetadata) {
	_ = state.Set(stateKeyMetadata, meta)
}

const (
	stateKeyMetadata        = "veadk.observability.metadata"
	stateKeyStreamingOutput = "veadk.observability.streaming_output"
	stateKeyStreamingSpan   = "veadk.observability.streaming_span"
	stateKeyAgentSpan       = "veadk.observability.agent_span"
	stateKeyAgentCtx        = "veadk.observability.agent_ctx"
	stateKeyInvocationSpan  = "veadk.observability.invocation_span"
	stateKeyInvocationCtx   = "veadk.observability.invocation_ctx"
)

// spanMetadata groups various observational data points in a single structure
// to keep the ADK State clean.
type spanMetadata struct {
	StartTime           time.Time
	FirstTokenTime      time.Time
	PromptTokens        int64
	CandidateTokens     int64
	TotalTokens         int64
	PrevPromptTokens    int64
	PrevCandidateTokens int64
	PrevTotalTokens     int64
	ModelName           string
}
