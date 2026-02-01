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
	"iter"
	"time"

	"github.com/volcengine/veadk-go/observability/exporter"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/session"
)

// ObservedLauncher wraps an adk launcher to provide automatic root span and context propagation.
type ObservedLauncher struct {
	launcher.Launcher
}

func NewObservedLauncher(base launcher.Launcher) launcher.Launcher {
	return &ObservedLauncher{Launcher: base}
}

func (l *ObservedLauncher) Execute(ctx context.Context, config *launcher.Config, args []string) error {
	// Attempt to get user/session from config or env if not present in context
	userID := GetUserId(ctx)
	sessionID := GetSessionId(ctx)

	// 1. Start the root 'invocation' span.
	tracedCtx, span := StartSpan(ctx, SpanInvocation)

	// Ensure span ends and all data is flushed before returning
	defer func() {
		sc := span.SpanContext()
		if sc.IsValid() {
			exporter.UnregisterInvocationSpanContext(sc.TraceID())
		}
		span.End()
		tp := otel.GetTracerProvider()
		if sdkTP, ok := tp.(*sdktrace.TracerProvider); ok {
			_ = sdkTP.ForceFlush(context.Background())
		}
	}()

	// 2. Set root attributes
	SetCommonAttributes(tracedCtx, span)
	SetWorkflowAttributes(span)
	if jsonIn, err := json.Marshal(args); err == nil {
		span.SetAttributes(attribute.String(AttrGenAIInputValue, string(jsonIn)))
	}

	// 3. Setup IDs in context
	tracedCtx = WithUserId(tracedCtx, userID)
	tracedCtx = WithSessionId(tracedCtx, sessionID)

	startTime := time.Now()

	// 4. Run the base launcher. This is usually a blocking call.
	err := l.Launcher.Execute(tracedCtx, config, args)

	// 5. Record final metrics
	elapsed := time.Since(startTime).Seconds()
	metricAttrs := []attribute.KeyValue{
		attribute.String("gen_ai_operation_name", "chain"),
		attribute.String("gen_ai_operation_type", "workflow"),
		attribute.String("gen_ai.system", "veadk"),
	}
	RecordOperationDuration(context.Background(), elapsed, metricAttrs...)
	RecordAPMPlusSpanLatency(context.Background(), elapsed, metricAttrs...)

	if err != nil {
		span.RecordError(err)
	}

	return err
}

// TraceRun is a helper to wrap runner.Run calls with an 'invocation' span.
// This is the root span for any GenAI request.
func TraceRun(ctx context.Context, userID, sessionID string, msg any, fn func(context.Context) iter.Seq2[*session.Event, error]) iter.Seq2[*session.Event, error] {
	ctx = WithUserId(ctx, userID)
	ctx = WithSessionId(ctx, sessionID)
	tracedCtx, span := StartSpan(ctx, SpanInvocation)

	// Use centralized attribute setting logic
	SetCommonAttributes(tracedCtx, span)
	SetWorkflowAttributes(span)

	if jsonIn, err := json.Marshal(msg); err == nil {
		span.SetAttributes(attribute.String(AttrGenAIInputValue, string(jsonIn)))
	}

	startTime := time.Now()

	return func(yield func(*session.Event, error) bool) {
		defer func() {
			elapsed := time.Since(startTime).Seconds()
			// Record root span metrics manually since processor is removed
			metricAttrs := []attribute.KeyValue{
				attribute.String("gen_ai_operation_name", "chain"),
				attribute.String("gen_ai_operation_type", "workflow"),
				attribute.String("gen_ai_system", "veadk"),
			}
			RecordOperationDuration(context.Background(), elapsed, metricAttrs...)
			RecordAPMPlusSpanLatency(context.Background(), elapsed, metricAttrs...)

			sc := span.SpanContext()
			if sc.IsValid() {
				exporter.UnregisterInvocationSpanContext(sc.TraceID())
			}
			span.End()

			// Force flush after ending the root span
			tp := otel.GetTracerProvider()
			if sdkTP, ok := tp.(*sdktrace.TracerProvider); ok {
				_ = sdkTP.ForceFlush(context.Background())
			}
		}()
		for event, err := range fn(tracedCtx) {
			if err != nil {
				span.RecordError(err)
			}
			if !yield(event, err) {
				return
			}
		}
	}
}

func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	tp := otel.GetTracerProvider()
	tr := tp.Tracer(InstrumentationName)
	// Use SpanKindServer for the root invocation span to mark it as an entry point
	kind := trace.SpanKindInternal
	if name == SpanInvocation {
		kind = trace.SpanKindServer
	}
	ctx, span := tr.Start(ctx, name, trace.WithSpanKind(kind))
	return ctx, span
}
