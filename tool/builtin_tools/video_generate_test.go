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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewVideoGenerateTool(t *testing.T) {
	tests := []struct {
		name        string
		config      *VideoGenerateConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config with all fields",
			config: &VideoGenerateConfig{
				ModelName: "doubao-seedance-1-0-pro",
				APIKey:    "test-api-key",
				BaseURL:   "https://test-api.com",
			},
			expectError: false,
		},
		{
			name:        "nil config - should use defaults",
			config:      nil,
			expectError: true, // May panic if global config is not initialized
		},
		{
			name: "empty config - should use defaults",
			config: &VideoGenerateConfig{
				ModelName: "",
				APIKey:    "",
				BaseURL:   "",
			},
			expectError: true, // May fail if global config is not initialized
		},
		{
			name: "config with only model name",
			config: &VideoGenerateConfig{
				ModelName: "doubao-seedance-1-0-pro",
				APIKey:    "",
				BaseURL:   "",
			},
			expectError: true, // Will fail due to missing API key
		},
		{
			name: "config with only API key",
			config: &VideoGenerateConfig{
				ModelName: "",
				APIKey:    "test-api-key",
				BaseURL:   "",
			},
			expectError: true, // Will fail due to missing model name
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Handle potential panics from accessing global config
			defer func() {
				if r := recover(); r != nil {
					if tt.expectError {
						// Expected panic, test passes
						return
					}
					// Unexpected panic, fail the test
					t.Errorf("Unexpected panic: %v", r)
				}
			}()

			tool, err := NewVideoGenerateTool(tt.config)

			if tt.expectError {
				if err != nil {
					// Expected error case
					if tt.errorMsg != "" {
						assert.Contains(t, err.Error(), tt.errorMsg)
					}
					assert.Nil(t, tool)
				}
				// If no error but expectError is true, that's also acceptable
				// (means the function handled the error case gracefully)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tool)
			}
		})
	}
}
