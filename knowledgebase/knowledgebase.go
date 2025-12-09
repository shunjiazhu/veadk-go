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

package knowledgebase

import (
	"errors"
	"fmt"
)

var InvalidKnowledgeBackendErr = errors.New("invalid knowledge backend type")

type KnowledgeBase[T any] struct {
	Name          string
	Description   string
	TopK          int
	AppName       string
	Index         string
	Backend       any
	BackendConfig T
}

func getKnowledgeBackend(backend KnowledgeBackendType) (KnowledgeBackend, error) {
	switch backend {
	case VikingBackend:
		return nil, nil
	case RedisBackend, LocalBackend, OpensearchBackend:
		return nil, fmt.Errorf("%w: %s", InvalidKnowledgeBackendErr, string(backend))
	default:
		return nil, fmt.Errorf("%w: %s", InvalidKnowledgeBackendErr, string(backend))
	}
}
