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
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"

	"github.com/volcengine/veadk-go/configs"
	"github.com/volcengine/veadk-go/log"
	"github.com/volcengine/veadk-go/observability/exporter"
)

// NewPlugin creates a new observability plugin for ADK.
func NewPlugin(opts ...Option) *plugin.Plugin {
	observabilityConfig := configs.GetGlobalConfig().Observability.Clone()
	for _, opt := range opts {
		opt(observabilityConfig)
	}

	p := &observabilityPlugin{
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

type Option func(config *configs.ObservabilityConfig)

func WithEnableMetrics(enable bool) Option {
	return func(cfg *configs.ObservabilityConfig) {
		enableVal := enable
		cfg.OpenTelemetry.EnableMetrics = &enableVal
	}
}

type observabilityPlugin struct {
	config *configs.ObservabilityConfig
	tracer trace.Tracer
}

func (p *observabilityPlugin) isMetricsEnabled() bool {
	if p.config == nil || p.config.OpenTelemetry == nil {
		return false
	}
	if p.config.OpenTelemetry.EnableMetrics != nil {
		return *p.config.OpenTelemetry.EnableMetrics
	}
	return false
}

func (p *observabilityPlugin) BeforeRun(ctx agent.InvocationContext) (*genai.Content, error) {
	newCtx, span := p.tracer.Start(context.Context(ctx), SpanInvocation, trace.WithSpanKind(trace.SpanKindServer))

	_ = ctx.Session().State().Set(keyInvocationSpan, span)
	_ = ctx.Session().State().Set(keyInvocationCtx, newCtx)

	sc := span.SpanContext()
	if sc.IsValid() {
		exporter.RegisterInvocationSpan(sc.TraceID(), span)
	}

	setCommonAttributes(newCtx, span)
	setWorkflowAttributes(span)

	meta := &spanMetadata{StartTime: time.Now()}
	p.storeSpanMetadata(ctx.Session().State(), meta)

	return nil, nil
}

func (p *observabilityPlugin) AfterRun(ctx agent.InvocationContext) {
	if s, _ := ctx.Session().State().Get(keyInvocationSpan); s != nil {
		span := s.(trace.Span)
		if span.IsRecording() {
			sc := span.SpanContext()
			if sc.IsValid() {
				exporter.UnregisterInvocationSpan(sc.TraceID())
				exporter.UnregisterAgentSpanContext(sc.TraceID())
				exporter.UnregisterLLMSpanContext(sc.TraceID())
			}
			span.End()
		}
	}
}

func (p *observabilityPlugin) BeforeModel(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
	parentCtx := context.Context(ctx)
	if actx, _ := ctx.State().Get(keyAgentCtx); actx != nil {
		parentCtx = actx.(context.Context)
	} else if ictx, _ := ctx.State().Get(keyInvocationCtx); ictx != nil {
		parentCtx = ictx.(context.Context)
	}

	newCtx, span := p.tracer.Start(parentCtx, SpanCallLLM)
	_ = ctx.State().Set(keyStreamingSpan, span)
	_ = ctx.State().Set(keyModelStartTime, time.Now())

	sc := span.SpanContext()
	if sc.IsValid() {
		exporter.RegisterLLMSpanContext(sc.TraceID(), sc)
	}

	meta := p.getSpanMetadata(ctx.State())
	meta.PrevPromptTokens = meta.PromptTokens
	meta.PrevCandidateTokens = meta.CandidateTokens
	meta.PrevTotalTokens = meta.TotalTokens
	meta.ModelName = req.Model
	p.storeSpanMetadata(ctx.State(), meta)

	setCommonAttributes(newCtx, span)
	setLLMAttributes(span)

	messages := p.extractMessages(req)
	messagesJSON, _ := json.Marshal(messages)

	attrs := []attribute.KeyValue{
		attribute.String(AttrGenAIRequestModel, req.Model),
		attribute.String(AttrGenAIRequestType, "chat"),
		attribute.String(AttrGenAISystem, GetModelProvider(context.Context(ctx))),
		attribute.String(AttrGenAIMessages, string(messagesJSON)),
		attribute.String(AttrGenAIInput, string(messagesJSON)),
	}
	attrs = append(attrs, p.flattenPrompt(messages)...)
	span.SetAttributes(attrs...)

	return nil, nil
}

func (p *observabilityPlugin) AfterModel(ctx agent.CallbackContext, resp *model.LLMResponse, err error) (*model.LLMResponse, error) {
	s, _ := ctx.State().Get(keyStreamingSpan)
	if s == nil {
		return nil, nil
	}
	span := s.(trace.Span)

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

	meta := p.getSpanMetadata(ctx.State())
	var finalModelName string
	if resp.CustomMetadata != nil {
		if m, ok := resp.CustomMetadata["response_model"].(string); ok {
			finalModelName = m
		}
	}
	if finalModelName == "" {
		finalModelName = meta.ModelName
	}
	if finalModelName != "" {
		span.SetAttributes(attribute.String(AttrGenAIResponseModel, finalModelName))
	}

	if resp.UsageMetadata != nil {
		p.handleUsage(ctx, span, resp, resp.Partial, finalModelName)
	}

	var currentAcc *genai.Content
	cached, _ := ctx.State().Get(keyStreamingOutput)
	if cached != nil {
		currentAcc = cached.(*genai.Content)
	}

	// Metrics: TTFT
	if resp.Partial && currentAcc == nil && resp.Content != nil {
		startTimeVal, _ := ctx.State().Get(keyModelStartTime)
		if st, ok := startTimeVal.(time.Time); ok {
			meta.FirstTokenTime = time.Now()
			p.storeSpanMetadata(ctx.State(), meta)

			if p.isMetricsEnabled() {
				latency := time.Since(st).Seconds()
				metricAttrs := []attribute.KeyValue{
					attribute.String(AttrGenAISystem, "veadk"),
					attribute.String("gen_ai_response_model", finalModelName),
					attribute.String("gen_ai_operation_name", "chat"),
				}
				RecordStreamingTimeToFirstToken(context.Context(ctx), latency, metricAttrs...)
			}
		}
	}

	if resp.Content != nil {
		if currentAcc == nil {
			currentAcc = &genai.Content{Role: resp.Content.Role}
		}
		for _, part := range resp.Content.Parts {
			newPart := &genai.Part{}
			*newPart = *part
			currentAcc.Parts = append(currentAcc.Parts, newPart)
		}
		_ = ctx.State().Set(keyStreamingOutput, currentAcc)
	}

	if !resp.Partial && currentAcc != nil {
		span.SetAttributes(attribute.String(AttrGenAIOutput, currentAcc.Parts[0].Text)) // Simplified
		span.SetAttributes(p.flattenCompletion(currentAcc)...)

		startTimeVal, _ := ctx.State().Get(keyModelStartTime)
		if st, ok := startTimeVal.(time.Time); ok {
			duration := time.Since(st).Seconds()
			if p.isMetricsEnabled() {
				metricAttrs := []attribute.KeyValue{
					attribute.String(AttrGenAISystem, "veadk"),
					attribute.String("gen_ai_response_model", finalModelName),
				}
				RecordOperationDuration(context.Context(ctx), duration, metricAttrs...)
			}
		}
	}

	return nil, nil
}

func (p *observabilityPlugin) handleUsage(ctx agent.CallbackContext, span trace.Span, resp *model.LLMResponse, isStream bool, modelName string) {
	meta := p.getSpanMetadata(ctx.State())

	currentPrompt := int64(resp.UsageMetadata.PromptTokenCount)
	currentCandidate := int64(resp.UsageMetadata.CandidatesTokenCount)
	currentTotal := int64(resp.UsageMetadata.TotalTokenCount)

	meta.PromptTokens = meta.PrevPromptTokens + currentPrompt
	meta.CandidateTokens = meta.PrevCandidateTokens + currentCandidate
	meta.TotalTokens = meta.PrevTotalTokens + currentTotal
	p.storeSpanMetadata(ctx.State(), meta)

	attrs := []attribute.KeyValue{
		attribute.Int64(AttrGenAIUsageInputTokens, currentPrompt),
		attribute.Int64(AttrGenAIUsageOutputTokens, currentCandidate),
		attribute.Int64(AttrGenAIUsageTotalTokens, currentTotal),
	}
	span.SetAttributes(attrs...)

	if p.isMetricsEnabled() && !isStream {
		metricAttrs := []attribute.KeyValue{
			attribute.String(AttrGenAISystem, "veadk"),
			attribute.String("gen_ai_response_model", modelName),
		}
		RecordTokenUsage(context.Context(ctx), currentPrompt, currentCandidate, metricAttrs...)
	}
}

func (p *observabilityPlugin) BeforeTool(ctx tool.Context, tool tool.Tool, args map[string]any) (map[string]any, error) {
	_ = ctx.State().Set(keyToolStartTime, time.Now())
	return nil, nil
}

func (p *observabilityPlugin) AfterTool(ctx tool.Context, tool tool.Tool, args, result map[string]any, err error) (map[string]any, error) {
	span := trace.SpanFromContext(context.Context(ctx))
	if !span.IsRecording() {
		return nil, nil
	}

	startTimeVal, _ := ctx.State().Get(keyToolStartTime)
	if st, ok := startTimeVal.(time.Time); ok {
		duration := time.Since(st).Seconds()
		if p.isMetricsEnabled() {
			metricAttrs := []attribute.KeyValue{
				attribute.String("gen_ai_operation_name", tool.Name()),
				attribute.String("gen_ai_operation_type", "tool"),
			}
			RecordOperationDuration(context.Context(ctx), duration, metricAttrs...)
		}
	}

	return nil, nil
}

func (p *observabilityPlugin) BeforeAgent(ctx agent.CallbackContext) (*genai.Content, error) {
	agentName := ctx.AgentName()
	parentCtx := context.Context(ctx)
	if ictx, _ := ctx.State().Get(keyInvocationCtx); ictx != nil {
		parentCtx = ictx.(context.Context)
	}

	newCtx, span := p.tracer.Start(parentCtx, SpanInvokeAgent+" "+agentName)

	_ = ctx.State().Set(keyAgentSpan, span)
	_ = ctx.State().Set(keyAgentCtx, newCtx)

	sc := span.SpanContext()
	if sc.IsValid() {
		exporter.RegisterAgentSpanContext(sc.TraceID(), sc)
	}

	setCommonAttributes(newCtx, span)
	setWorkflowAttributes(span)
	setAgentAttributes(span, agentName)

	return nil, nil
}

func (p *observabilityPlugin) AfterAgent(ctx agent.CallbackContext) (*genai.Content, error) {
	if s, _ := ctx.State().Get(keyAgentSpan); s != nil {
		span := s.(trace.Span)
		sc := span.SpanContext()
		if sc.IsValid() {
			exporter.UnregisterAgentSpanContext(sc.TraceID())
		}
		span.End()
	}
	return nil, nil
}
