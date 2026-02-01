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
	"encoding/json"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// extractMessages converts ADK model.LLMRequest contents into a JSON-compatible message list.
func (p *observabilityPlugin) extractMessages(req *model.LLMRequest) []map[string]any {
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
			msg["tool_responses"] = toolResponses
		}

		messages = append(messages, msg)
	}
	return messages
}

func (p *observabilityPlugin) flattenPrompt(messages []map[string]any) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	idx := 0
	for _, msg := range messages {
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

func (p *observabilityPlugin) flattenCompletion(content *genai.Content) []attribute.KeyValue {
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

func (p *observabilityPlugin) addUserMessageEvents(span trace.Span, ctx agent.CallbackContext, req *model.LLMRequest) {
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

			if len(attrs) > 0 {
				span.AddEvent("gen_ai.user.message", trace.WithAttributes(attrs...))
			}
		}
	}
}

func (p *observabilityPlugin) addChoiceEvents(span trace.Span, content *genai.Content) {
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
