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

package exporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/volcengine/veadk-go/configs"
)

func TestToObservabilityConfig(t *testing.T) {
	cfg := &configs.ObservabilityConfig{
		OpenTelemetry: &configs.OpenTelemetryConfig{
			ApmPlus: &configs.ApmPlusConfig{
				Endpoint:    "http://apmplus-example.com",
				APIKey:      "test-key",
				ServiceName: "test-service",
			},
			EnableGlobalTracer: true,
		},
	}

	expConfig, ok := ToObservabilityConfig(cfg)
	assert.True(t, ok)
	assert.NotNil(t, expConfig.ApmPlus)
	assert.Equal(t, "http://apmplus-example.com", expConfig.ApmPlus.Endpoint)
}

func TestToObservabilityConfig_Priority(t *testing.T) {
	// Nested priority check: CozeLoop > APMPlus
	cfg := &configs.ObservabilityConfig{
		OpenTelemetry: &configs.OpenTelemetryConfig{
			ApmPlus: &configs.ApmPlusConfig{
				Endpoint: "apm-endpoint",
			},
			CozeLoop: &configs.CozeLoopConfig{
				Endpoint: "coze-endpoint",
				APIKey:   "coze-key",
			},
		},
	}

	expConfig, ok := ToObservabilityConfig(cfg)
	assert.True(t, ok)
	assert.NotNil(t, expConfig.CozeLoop)
	assert.Equal(t, "coze-endpoint", expConfig.CozeLoop.Endpoint)
}
