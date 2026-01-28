package main

import (
	"context"
	"fmt"
	"log"

	"github.com/volcengine/veadk-go/common"
	"github.com/volcengine/veadk-go/model"
	"github.com/volcengine/veadk-go/observability"
	"github.com/volcengine/veadk-go/utils"
	"go.opentelemetry.io/otel"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/genai"
)

func main() {
	ctx := context.Background()
	// Initialize observability
	if err := observability.Init(ctx); err != nil {
		log.Fatalf("observability.Init failed: %v", err)
	}
	defer func() {
		_ = observability.Shutdown(ctx)
	}()

	// Create model configuration
	cfg := &model.ClientConfig{
		APIKey:  utils.GetEnvWithDefault(common.MODEL_AGENT_API_KEY),
		BaseURL: utils.GetEnvWithDefault(common.MODEL_AGENT_API_BASE),
	}

	m, err := model.NewOpenAIModel(ctx, common.DEFAULT_MODEL_AGENT_NAME, cfg)
	if err != nil {
		log.Fatalf("NewOpenAIModel failed: %v", err)
	}

	fmt.Println("--- Sending Unary (Non-Streaming) Request ---")

	// Create a span manually to ensure metrics are recorded by SpanEnrichmentProcessor.OnEnd
	tracer := otel.Tracer(observability.InstrumentationName)
	ctx, span := tracer.Start(ctx, observability.SpanCallLLM)
	defer span.End()

	req := &adkmodel.LLMRequest{
		Contents: []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "Say 'Hello, unary!' and give me a brief fact about Go."},
				},
			},
		},
	}

	// Call GenerateContent with stream=false
	next := m.GenerateContent(ctx, req, false)

	fmt.Print("Response: ")
	for resp, err := range next {
		if err != nil {
			log.Fatalf("GenerateContent failed: %v", err)
		}
		if resp != nil && resp.Content != nil && len(resp.Content.Parts) > 0 {
			fmt.Print(resp.Content.Parts[0].Text)
		}
		if resp != nil && resp.UsageMetadata != nil {
			fmt.Printf("\n\nUsage: Input=%d, Output=%d, Total=%d\n",
				resp.UsageMetadata.PromptTokenCount,
				resp.UsageMetadata.CandidatesTokenCount,
				resp.UsageMetadata.TotalTokenCount)
		}
	}

	fmt.Println("\nWaiting for metrics to be flushed to console...")
}
