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
	"testing"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
	"github.com/volcengine/veadk-go/configs"
	"github.com/volcengine/veadk-go/observability/exporter"
)

// ... existing TestInitWithConfig and TestRegisterExporter ...

func TestNewExporter(t *testing.T) {
	ctx := context.Background()

	t.Run("Stdout", func(t *testing.T) {
		cfg := exporter.Config{ExporterType: exporter.ExporterStdout}
		exp, err := NewExporter(ctx, cfg)
		assert.NoError(t, err)
		assert.NotNil(t, exp)
	})

	t.Run("Multi-Platform", func(t *testing.T) {
		cfg := exporter.Config{
			CozeLoop: &exporter.CozeLoopConfig{APIKey: "test"},
			ApmPlus:  &exporter.ApmPlusConfig{APIKey: "test"},
		}
		exp, err := NewExporter(ctx, cfg)
		assert.NoError(t, err)
		assert.NotNil(t, exp)
	})

	t.Run("NoConfig", func(t *testing.T) {
		cfg := exporter.Config{}
		exp, err := NewExporter(ctx, cfg)
		assert.Error(t, err)
		assert.Nil(t, exp)
	})
}

func TestInit(t *testing.T) {
	mockey.PatchConvey("TestInit", t, func() {
		cfg := &configs.VeADKConfig{
			Observability: &configs.ObservabilityConfig{
				OpenTelemetry: &configs.OpenTelemetryConfig{
					CozeLoop: &configs.CozeLoopConfig{
						APIKey:      "test",
						ServiceName: "test",
					},
				},
			},
		}
		mockey.Mock(configs.GetGlobalConfig).Return(cfg).Build()

		err := Init(context.Background())
		assert.NoError(t, err)
	})
}
