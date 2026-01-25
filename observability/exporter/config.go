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
// It is intended to be populated by the caller (e.g., the configs package).
type Config struct {
	// ExporterType specifies which platform to use.
	ExporterType ExporterType

	// Endpoint is the target URL for the OTLP
	Endpoint string

	// APIKey is the authentication token required by the platform.
	APIKey string

	// ServiceName is the name identifying the application or workspace.
	ServiceName string

	// Region is used for volcano-specific services like TLS.
	Region string

	// FilePath is the target file path for ExporterFile.
	FilePath string

	// EnableGlobalTracer determines if the exporter should also be registered
	// to the global OpenTelemetry TracerProvider.
	EnableGlobalTracer bool
}

// ToObservabilityConfig converts the user-facing configuration to the package-agnostic struct.
// It returns an Config and a boolean indicating if a valid config was found.
func ToObservabilityConfig(c *configs.ObservabilityConfig) (Config, bool) {
	if c.OpenTelemetry == nil {
		return Config{}, false
	}
	ot := c.OpenTelemetry

	// Priority: CozeLoop > APMPlus > TLS
	if ot.CozeLoop != nil && (ot.CozeLoop.APIKey != "" || ot.CozeLoop.Endpoint != "") {
		return Config{
			ExporterType:       ExporterCozeLoop,
			Endpoint:           ot.CozeLoop.Endpoint,
			APIKey:             ot.CozeLoop.APIKey,
			ServiceName:        ot.CozeLoop.ServiceName,
			EnableGlobalTracer: ot.EnableGlobalTracer,
		}, true
	}
	if ot.ApmPlus != nil && (ot.ApmPlus.APIKey != "" || ot.ApmPlus.Endpoint != "") {
		return Config{
			ExporterType:       ExporterAPMPlus,
			Endpoint:           ot.ApmPlus.Endpoint,
			APIKey:             ot.ApmPlus.APIKey,
			ServiceName:        ot.ApmPlus.ServiceName,
			EnableGlobalTracer: ot.EnableGlobalTracer,
		}, true
	}
	if ot.TLS != nil && (ot.TLS.APIKey != "" || ot.TLS.Endpoint != "") {
		return Config{
			ExporterType:       ExporterTLS,
			Endpoint:           ot.TLS.Endpoint,
			APIKey:             ot.TLS.APIKey,
			ServiceName:        ot.TLS.ServiceName,
			Region:             ot.TLS.Region,
			EnableGlobalTracer: ot.EnableGlobalTracer,
		}, true
	}
	if ot.File != nil && ot.File.Path != "" {
		return Config{
			ExporterType:       ExporterFile,
			FilePath:           ot.File.Path,
			EnableGlobalTracer: ot.EnableGlobalTracer,
		}, true
	}
	return Config{}, false
}
