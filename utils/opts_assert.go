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

package utils

import (
	"errors"
)

var (
	OptsNIlErr        = errors.New("opts is nil")
	OptsInvalidKeyErr = errors.New("the key not in opts")
	OptsAssertTypeErr = errors.New("opts assert type error")
)

func ExtractOptsValue[T any](key string, opts ...map[string]any) (T, error) {
	var t T
	if opts == nil || len(opts) == 0 {
		return t, OptsNIlErr
	}
	for _, opt := range opts {
		val, ok := opt[key]
		if !ok {
			return t, OptsInvalidKeyErr
		}
		res, ok := val.(T)
		if !ok {
			return t, OptsAssertTypeErr
		}
		return res, nil
	}
	return t, OptsInvalidKeyErr
}

func ExtractOptsValueWithDefault[T any](key string, defaultVal T, opts ...map[string]any) T {
	if opts == nil {
		return defaultVal
	}
	for _, opt := range opts {
		val, ok := opt[key]
		if !ok {
			return defaultVal
		}
		res, ok := val.(T)
		if !ok {
			return defaultVal
		}
		return res
	}
	return defaultVal
}
