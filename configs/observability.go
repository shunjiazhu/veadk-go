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

package configs

import (
	"github.com/volcengine/veadk-go/utils"
)

// ObservabilityConfig groups specific configurations for different platforms.
type ObservabilityConfig struct {
	OpenTelemetry *OpenTelemetryConfig `yaml:"opentelemetry"`
}

type OpenTelemetryConfig struct {
	ApmPlus            *ApmPlusConfig     `yaml:"apmplus"`
	CozeLoop           *CozeLoopConfig    `yaml:"cozeloop"`
	TLS                *TLSExporterConfig `yaml:"tls"`
	File               *FileConfig        `yaml:"file"`
	EnableGlobalTracer bool               `yaml:"enable_global_tracer"`
}

type ApmPlusConfig struct {
	Endpoint    string `yaml:"endpoint"`
	APIKey      string `yaml:"api_key"`
	ServiceName string `yaml:"service_name"`
}

type CozeLoopConfig struct {
	Endpoint    string `yaml:"endpoint"`
	APIKey      string `yaml:"api_key"`
	ServiceName string `yaml:"service_name"`
}

type TLSExporterConfig struct {
	Endpoint    string `yaml:"endpoint"`
	APIKey      string `yaml:"api_key"`
	ServiceName string `yaml:"service_name"`
	Region      string `yaml:"region"`
}

type FileConfig struct {
	Path string `yaml:"path"`
}

func (c *ObservabilityConfig) MapEnvToConfig() {
	if c.OpenTelemetry == nil {
		c.OpenTelemetry = &OpenTelemetryConfig{}
	}
	ot := c.OpenTelemetry

	// APMPlus
	if v := utils.GetEnvWithDefault("OBSERVABILITY_OPENTELEMETRY_APMPLUS_ENDPOINT"); v != "" {
		if ot.ApmPlus == nil {
			ot.ApmPlus = &ApmPlusConfig{}
		}
		ot.ApmPlus.Endpoint = v
	}
	if v := utils.GetEnvWithDefault("OBSERVABILITY_OPENTELEMETRY_APMPLUS_API_KEY"); v != "" {
		if ot.ApmPlus == nil {
			ot.ApmPlus = &ApmPlusConfig{}
		}
		ot.ApmPlus.APIKey = v
	}
	if v := utils.GetEnvWithDefault("OBSERVABILITY_OPENTELEMETRY_APMPLUS_SERVICE_NAME"); v != "" {
		if ot.ApmPlus == nil {
			ot.ApmPlus = &ApmPlusConfig{}
		}
		ot.ApmPlus.ServiceName = v
	}

	// CozeLoop
	if v := utils.GetEnvWithDefault("OBSERVABILITY_OPENTELEMETRY_COZELOOP_ENDPOINT"); v != "" {
		if ot.CozeLoop == nil {
			ot.CozeLoop = &CozeLoopConfig{}
		}
		ot.CozeLoop.Endpoint = v
	}
	if v := utils.GetEnvWithDefault("OBSERVABILITY_OPENTELEMETRY_COZELOOP_API_KEY"); v != "" {
		if ot.CozeLoop == nil {
			ot.CozeLoop = &CozeLoopConfig{}
		}
		ot.CozeLoop.APIKey = v
	}
	if v := utils.GetEnvWithDefault("OBSERVABILITY_OPENTELEMETRY_COZELOOP_SERVICE_NAME"); v != "" {
		if ot.CozeLoop == nil {
			ot.CozeLoop = &CozeLoopConfig{}
		}
		ot.CozeLoop.ServiceName = v
	}

	// TLS
	if v := utils.GetEnvWithDefault("OBSERVABILITY_OPENTELEMETRY_TLS_ENDPOINT"); v != "" {
		if ot.TLS == nil {
			ot.TLS = &TLSExporterConfig{}
		}
		ot.TLS.Endpoint = v
	}
	if v := utils.GetEnvWithDefault("OBSERVABILITY_OPENTELEMETRY_TLS_API_KEY"); v != "" {
		if ot.TLS == nil {
			ot.TLS = &TLSExporterConfig{}
		}
		ot.TLS.APIKey = v
	}
	if v := utils.GetEnvWithDefault("OBSERVABILITY_OPENTELEMETRY_TLS_SERVICE_NAME"); v != "" {
		if ot.TLS == nil {
			ot.TLS = &TLSExporterConfig{}
		}
		ot.TLS.ServiceName = v
	}
	if v := utils.GetEnvWithDefault("OBSERVABILITY_OPENTELEMETRY_TLS_REGION"); v != "" {
		if ot.TLS == nil {
			ot.TLS = &TLSExporterConfig{}
		}
		ot.TLS.Region = v
	}

	// File
	if v := utils.GetEnvWithDefault("OBSERVABILITY_OPENTELEMETRY_FILE_PATH"); v != "" {
		if ot.File == nil {
			ot.File = &FileConfig{}
		}
		ot.File.Path = v
	}

	// Global Tracer
	if v := utils.GetEnvWithDefault("OBSERVABILITY_OPENTELEMETRY_ENABLE_GLOBAL_TRACER"); v != "" {
		ot.EnableGlobalTracer = v == "true"
	} else if v := utils.GetEnvWithDefault("OBSERVABILITY_ENABLE_GLOBAL_TRACER"); v != "" {
		// Fallback to legacy env var
		ot.EnableGlobalTracer = v == "true"
	}
}
