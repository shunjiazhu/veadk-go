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
	"github.com/volcengine/veadk-go/configs"
)

// ExporterType defines the target platform for exporting traces.
type ExporterType string

const (
	ExporterStdout   ExporterType = "stdout"
	ExporterFile     ExporterType = "file"
	ExporterAPMPlus  ExporterType = "apmplus"
	ExporterCozeLoop ExporterType = "cozeloop"
	ExporterTLS      ExporterType = "tls"
)

// Config is a package-agnostic configuration for initializing observability.
type Config struct {
	// ExporterType specifies which platform to use explicitly (like Stdout/File).
	ExporterType ExporterType

	// Platform configs (Can be multiple for multi-writing)
	ApmPlus  *ApmPlusConfig
	CozeLoop *CozeLoopConfig
	TLS      *TLSExporterConfig

	// FilePath is the target file path for ExporterFile.
	FilePath string

	// EnableGlobalTracer determines if the exporter should also be registered
	// to the global OpenTelemetry TracerProvider.
	EnableGlobalTracer bool

	// ServiceName is the primary service name used for the tracer and resource.
	ServiceName string
}

type ApmPlusConfig struct {
	Endpoint    string
	APIKey      string
	ServiceName string
}

type CozeLoopConfig struct {
	Endpoint    string
	APIKey      string
	ServiceName string
}

type TLSExporterConfig struct {
	Endpoint    string
	APIKey      string
	ServiceName string
	Region      string
}

// ToObservabilityConfig converts the user-facing configuration to the package-agnostic struct.
// Now it supports gathering all valid platform configs for simultaneous exporting.
func ToObservabilityConfig(c *configs.ObservabilityConfig) (Config, bool) {
	if c.OpenTelemetry == nil {
		return Config{}, false
	}
	ot := c.OpenTelemetry

	cfg := Config{
		EnableGlobalTracer: ot.EnableGlobalTracer,
	}

	if ot.File != nil && ot.File.Path != "" {
		cfg.ExporterType = ExporterFile
		cfg.FilePath = ot.File.Path
	}

	hasPlatform := false
	if ot.CozeLoop != nil && (ot.CozeLoop.APIKey != "" || ot.CozeLoop.Endpoint != "") {
		cfg.CozeLoop = &CozeLoopConfig{
			Endpoint:    ot.CozeLoop.Endpoint,
			APIKey:      ot.CozeLoop.APIKey,
			ServiceName: ot.CozeLoop.ServiceName,
		}
		if cfg.ServiceName == "" {
			cfg.ServiceName = ot.CozeLoop.ServiceName
		}
		hasPlatform = true
	}
	if ot.ApmPlus != nil && (ot.ApmPlus.APIKey != "" || ot.ApmPlus.Endpoint != "") {
		cfg.ApmPlus = &ApmPlusConfig{
			Endpoint:    ot.ApmPlus.Endpoint,
			APIKey:      ot.ApmPlus.APIKey,
			ServiceName: ot.ApmPlus.ServiceName,
		}
		if cfg.ServiceName == "" {
			cfg.ServiceName = ot.ApmPlus.ServiceName
		}
		hasPlatform = true
	}
	if ot.TLS != nil && (ot.TLS.APIKey != "" || ot.TLS.Endpoint != "") {
		cfg.TLS = &TLSExporterConfig{
			Endpoint:    ot.TLS.Endpoint,
			APIKey:      ot.TLS.APIKey,
			ServiceName: ot.TLS.ServiceName,
			Region:      ot.TLS.Region,
		}
		if cfg.ServiceName == "" {
			cfg.ServiceName = ot.TLS.ServiceName
		}
		hasPlatform = true
	}

	valid := hasPlatform || cfg.ExporterType != ""
	return cfg, valid
}
