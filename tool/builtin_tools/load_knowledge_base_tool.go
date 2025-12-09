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

package builtin_tools

import (
	"github.com/volcengine/veadk-go/knowledgebase"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type QueryKnowledgeReq struct {
	Query string `json:"query" jsonschema:"The query for loading the knowledge base for this tool"`
}

type WebSearchResult struct {
	Result []knowledgebase.KnowledgeEntry `json:"result,omitempty"`
}

// LoadKnowledgeBaseTool
// Loads the knowledgebase tool from a backend.
// Args:
// query: The query to load the knowledgebase for.
// Returns:
// A list of knowledge base results.
func LoadKnowledgeBaseTool(backend knowledgebase.KnowledgeBackend) (tool.Tool, error) {
	handler := func(ctx tool.Context, req *QueryKnowledgeReq) (WebSearchResult, error) {
		result, err := backend.Search(req.Query)
		if err != nil {
			return WebSearchResult{}, err
		}
		return WebSearchResult{
			Result: result,
		}, nil

	}
	return functiontool.New(
		functiontool.Config{
			Name:        "knowledge_base",
			Description: `This is a knowledge base. You can use it to answer questions. If any questions need you to look up the knowledge base, you should call knowledge_base function with a query.`,
		},
		handler)
}
