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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateLogClient(t *testing.T) {
	ctx := context.Background()
	url := "http://localhost:4317"
	headers := map[string]string{"test": "header"}

	t.Run("default protocol", func(t *testing.T) {
		exporter, err := createLogClient(ctx, url, "", headers)
		assert.NoError(t, err)
		assert.NotNil(t, exporter)
	})

	t.Run("http protocol", func(t *testing.T) {
		exporter, err := createLogClient(ctx, url, "http/protobuf", headers)
		assert.NoError(t, err)
		assert.NotNil(t, exporter)
	})

	t.Run("env protocol", func(t *testing.T) {
		os.Setenv(OTELExporterOTLPProtocolEnvKey, "http/json")
		defer os.Unsetenv(OTELExporterOTLPProtocolEnvKey)

		exporter, err := createLogClient(ctx, url, "", headers)
		assert.NoError(t, err)
		assert.NotNil(t, exporter)
	})

	t.Run("missing url", func(t *testing.T) {
		exporter, err := createLogClient(ctx, "", "", headers)
		assert.Error(t, err)
		assert.Nil(t, exporter)
		assert.Equal(t, "OTEL_EXPORTER_OTLP_ENDPOINT is not set", err.Error())
	})
}

func TestCreateTraceClient(t *testing.T) {
	ctx := context.Background()
	url := "http://localhost:4317"
	headers := map[string]string{"test": "header"}

	t.Run("http protocol", func(t *testing.T) {
		exporter, err := createTraceClient(ctx, url, "http/protobuf", headers)
		assert.NoError(t, err)
		assert.NotNil(t, exporter)
	})

	t.Run("grpc protocol", func(t *testing.T) {
		exporter, err := createTraceClient(ctx, url, "grpc", headers)
		assert.NoError(t, err)
		assert.NotNil(t, exporter)
	})
}

func TestCreateMetricClient(t *testing.T) {
	ctx := context.Background()
	url := "http://localhost:4317"
	headers := map[string]string{"test": "header"}

	t.Run("http protocol", func(t *testing.T) {
		exporter, err := createMetricClient(ctx, url, "http/protobuf", headers)
		assert.NoError(t, err)
		assert.NotNil(t, exporter)
	})

	t.Run("grpc protocol", func(t *testing.T) {
		exporter, err := createMetricClient(ctx, url, "grpc", headers)
		assert.NoError(t, err)
		assert.NotNil(t, exporter)
	})
}
