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

package long_term_memory_backends

import (
	"context"
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/veadk-go/integrations/ve_viking"
	"github.com/volcengine/veadk-go/integrations/ve_viking/viking_memory"
	"github.com/volcengine/veadk-go/utils"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

func TestNewVikingDbMemoryBackend(t *testing.T) {
	mockey.PatchConvey("TestNewVikingDbMemoryBackend", t, func() {
		mockey.Mock(ve_viking.NewConfig).Return(&ve_viking.ClientConfig{}, nil).Build()
		mockey.PatchConvey("collection info failed", func() {
			mockey.Mock((*viking_memory.Client).CollectionInfo).Return(errors.New("collection info failed")).Build()
			memoryService, err := NewVikingDbMemoryBackend(&VikingDbMemoryConfig{})
			assert.Nil(t, memoryService)
			assert.NotNil(t, err)
		})
		mockey.PatchConvey("collection info failed and not create", func() {
			mockey.Mock((*viking_memory.Client).CollectionInfo).Return(errors.New("collection not exist")).Build()
			createNotExist := new(bool)
			*createNotExist = false
			memoryService, err := NewVikingDbMemoryBackend(&VikingDbMemoryConfig{CreateIfNotExist: createNotExist})
			assert.Nil(t, memoryService)
			assert.NotNil(t, err)
		})

		mockey.PatchConvey("collection info failed and create failed", func() {
			mockey.Mock((*viking_memory.Client).CollectionInfo).Return(errors.New("collection not exist")).Build()
			mockey.Mock((*viking_memory.Client).CollectionCreate).Return(nil, errors.New("collection info failed")).Build()
			memoryService, err := NewVikingDbMemoryBackend(&VikingDbMemoryConfig{})
			assert.Nil(t, memoryService)
			assert.NotNil(t, err)
		})

		mockey.PatchConvey("collection info failed and create success", func() {
			mockey.Mock((*viking_memory.Client).CollectionInfo).Return(errors.New("collection not exist")).Build()
			mockey.Mock((*viking_memory.Client).CollectionCreate).Return(&ve_viking.CommonResponse{Code: ve_viking.VikingKnowledgeBaseSuccessCode}, nil).Build()
			memoryService, err := NewVikingDbMemoryBackend(&VikingDbMemoryConfig{})
			assert.NotNil(t, memoryService)
			assert.Nil(t, err)
		})

		mockey.PatchConvey("collection info success", func() {
			mockey.Mock((*viking_memory.Client).CollectionInfo).Return(nil).Build()
			memoryService, err := NewVikingDbMemoryBackend(&VikingDbMemoryConfig{})
			assert.NotNil(t, memoryService)
			assert.Nil(t, err)
		})
	})
}

type TestSession struct {
	session.Session
}

type TestEvents struct {
	Events []*session.Event
}

func (t *TestEvents) All() iter.Seq[*session.Event] {
	return nil
}

func (t *TestEvents) Len() int {
	return len(t.Events)
}

func (t *TestEvents) At(i int) *session.Event {
	return t.Events[i]
}

func TestVikingDbMemoryBackend_AddSession(t *testing.T) {
	v := &VikingDBMemoryBackend{
		client: &viking_memory.Client{},
		config: &VikingDbMemoryConfig{
			Index: DefaultIndex,
		},
	}
	ctx := context.Background()
	mockey.PatchConvey("TestVikingDbMemoryBackend_AddSession", t, func() {
		mockey.Mock((*TestSession).Events).Return(&TestEvents{
			Events: []*session.Event{
				{
					LLMResponse: model.LLMResponse{
						Content: &genai.Content{
							Parts: []*genai.Part{},
							Role:  "user",
						},
					},
				},
				{
					LLMResponse: model.LLMResponse{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{
									Text: "",
								},
							},
							Role: "user",
						},
					},
				},
				{
					LLMResponse: model.LLMResponse{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{
									Text: "I am model",
								},
							},
							Role: "model",
						},
					},
				},
				{
					LLMResponse: model.LLMResponse{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{
									Text: "I am user",
								},
							},
							Role: "user",
						},
					},
				},
			},
		}).Build()
		mockey.Mock((*TestSession).UserID).Return("test").Build()
		mockey.Mock((*TestSession).LastUpdateTime).Return(time.Now()).Build()
		mockey.Mock((*viking_memory.Client).AddSession).Return(&ve_viking.CommonResponse{Code: ve_viking.VikingKnowledgeBaseSuccessCode}, nil).Build()
		err := v.AddSession(ctx, &TestSession{})
		assert.Nil(t, err)
	})
}

func TestVikingDbMemoryBackend_Search(t *testing.T) {
	v := &VikingDBMemoryBackend{
		client: &viking_memory.Client{},
		config: &VikingDbMemoryConfig{
			Index: DefaultIndex,
		},
	}
	ctx := context.Background()
	nowUnix := time.Now().UnixMilli()
	now := utils.ConvertTimeMillToTime(nowUnix)
	mockey.PatchConvey("TestVikingDbMemoryBackend_Search", t, func() {
		mockey.Mock((*viking_memory.Client).CollectionSearchMemory).Return(&viking_memory.CollectionSearchMemoryResponse{
			Code: ve_viking.VikingKnowledgeBaseSuccessCode,
			Data: &viking_memory.CollectionSearchMemoryResponseData{
				ResultList: []*viking_memory.CollectionSearchResponseItem{
					{
						MemoryInfo: &viking_memory.MemoryInfo{
							Summary: "test1",
						},
						Time: nowUnix,
					},
					{
						MemoryInfo: &viking_memory.MemoryInfo{
							Summary: "test2",
						},
						Time: nowUnix,
					},
				},
			},
		}, nil).Build()
		resp, err := v.Search(ctx, &memory.SearchRequest{
			UserID: "test",
			Query:  "test",
		})
		assert.Nil(t, err)
		assert.Equal(t, 2, len(resp.Memories))
		for i, v := range resp.Memories {
			assert.Equal(t, now, v.Timestamp)
			assert.Equal(t, "user", v.Author)
			if i == 0 {
				assert.Equal(t, "test1", v.Content.Parts[0].Text)
			} else {
				assert.Equal(t, "test2", v.Content.Parts[0].Text)
			}
			assert.Equal(t, "user", v.Content.Role)
		}
	})
}
