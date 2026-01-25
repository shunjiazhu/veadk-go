# VeADK Go Observability Package

This package provides observability features for the VeADK Go SDK, aligned with the [VeADK Python SDK](https://volcengine.github.io/veadk-python/observation/span-attributes/) and [OpenTelemetry GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/).

## Features

- **Protocol Alignment**: Standardizes Google ADK internal spans to industry-standard GenAI span attributes.
- **Multi-Platform Support**: Simultaneously export traces to CozeLoop, APMPlus, Volcano TLS, or local files/stdout.
- **Zero-Config Instrumentation**: Automatically captures `SessionID`, `UserID`, and `AppName` from context, global config, or environment variables.
- **Launcher Integration**: Specialized `ObservedLauncher` to ensure complete trace capture from the root invocation.
- **Metric Support**: Automated recording of token usage and operation latencies.

## Configuration

### YAML Configuration

Add an `observability` section to your `config.yaml`:

```yaml
observability:
  opentelemetry:
    enable_global_tracer: true
    cozeloop:
      endpoint: "https://api.coze.cn/v1/loop/opentelemetry/v1/traces"
      api_key: "YOUR_COZE_API_KEY"
      service_name: "YOUR_COZE_SPACE_ID"
    apmplus:
      endpoint: "apmplus-cn-beijing.volces.com:4317"
      api_key: "YOUR_APMPLUS_API_KEY"
      service_name: "YOUR_SERVICE_NAME"
```

### Environment Variables

All settings can be overridden via environment variables:

- `OBSERVABILITY_OPENTELEMETRY_COZELOOP_API_KEY`
- `OBSERVABILITY_OPENTELEMETRY_APMPLUS_API_KEY`
- `OBSERVABILITY_OPENTELEMETRY_ENABLE_GLOBAL_TRACER` (default: false)

## Usage

### Simple Initialization

The easiest way to start is using the global configuration:

```go
import "github.com/volcengine/veadk-go/observability"

func main() {
    ctx := context.Background()
    // Initializes exporters based on config.yaml or env vars
    if err := observability.Init(ctx); err != nil {
        log.Printf("Failed to init observability: %v", err)
    }
}
```

### Launcher Integration

To ensure the root span (e.g., `invocation`) is captured correctly, wrap your launcher:

```go
import (
    "github.com/volcengine/veadk-go/observability"
    "google.golang.org/adk/cmd/launcher/full"
)

func main() {
    // ... setup config ...
    
    // Wrap the standard launcher
    l := observability.NewObservedLauncher(full.NewLauncher())
    
    if err := l.Execute(ctx, config, os.Args[1:]); err != nil {
        log.Fatal(err)
    }
}
```
