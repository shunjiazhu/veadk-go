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

func TestContextAttributes(t *testing.T) {
	ctx := context.Background()

	t.Run("SessionId", func(t *testing.T) {
		assert.Equal(t, "", GetSessionId(ctx))
		ctxWithId := WithSessionId(ctx, "test-session")
		assert.Equal(t, "test-session", GetSessionId(ctxWithId))
	})

	t.Run("UserId", func(t *testing.T) {
		assert.Equal(t, "", GetUserId(ctx))
		ctxWithId := WithUserId(ctx, "test-user")
		assert.Equal(t, "test-user", GetUserId(ctxWithId))
	})

	t.Run("AppName", func(t *testing.T) {
		assert.Equal(t, "", GetAppName(ctx))
		ctxWithName := WithAppName(ctx, "test-app")
		assert.Equal(t, "test-app", GetAppName(ctxWithName))
	})
}

func TestEnvFallback(t *testing.T) {
	os.Setenv(EnvAppName, "env-app")
	defer os.Unsetenv(EnvAppName)

	ctx := context.Background()
	assert.Equal(t, "env-app", GetAppName(ctx))

	// Context should still win
	ctxWithApp := WithAppName(ctx, "ctx-app")
	assert.Equal(t, "ctx-app", GetAppName(ctxWithApp))
}
