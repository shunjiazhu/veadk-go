# VeADK Go Observability Package

This package provides comprehensive observability features for the VeADK Go SDK, fully aligned with the [VeADK Python SDK](https://volcengine.github.io/veadk-python/observation/span-attributes/) and [OpenTelemetry GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/).

## Features

- **Full Python ADK Alignment**: Implements the same span attributes, event structures, and naming conventions as the Python ADK
- **Multi-Platform Support**: Simultaneously export traces to CozeLoop, APMPlus, Volcano TLS, or local files/stdout
- **Automatic Attribute Enrichment**: Automatically captures and propagates `SessionID`, `UserID`, `AppName`, `InvocationID` from context, config, or environment
- **Span Hierarchy Support**: Properly tracks invocation → agent → LLM/tool execution hierarchies
- **Launcher Integration**: Specialized `ObservedLauncher` ensures complete trace capture from root invocation span
- **Metrics Support**: Automated recording of token usage, operation latencies, and first token latency

## Span Attribute Specification

VeADK Go implements the following span attribute categories as documented in [Python ADK Span Attributes](https://volcengine.github.io/veadk-python/observation/span-attributes/):

### Common Attributes (All Spans)
- `gen_ai.system` - Model provider (e.g., "openai", "ark")
- `gen_ai.system.version` - VeADK version
- `gen_ai.agent.name` - Agent name
- `gen_ai.app.name` / `app_name` / `app.name` - Application name
- `gen_ai.user.id` / `user.id` - User identifier
- `gen_ai.session.id` / `session.id` - Session identifier
- `gen_ai.invocation.id` / `invocation.id` - Invocation identifier
- `cozeloop.report.source` - Fixed value "veadk"
- `cozeloop.call_type` - Call type for CozeLoop
- `openinference.instrumentation.veadk` - Instrumentation version

### LLM Span Attributes
- `gen_ai.span.kind` - "llm"
- `gen_ai.operation.name` - "chat"
- `gen_ai.request.model` - Model name
- `gen_ai.request.type` - Request type
- `gen_ai.request.max_tokens` - Max output tokens
- `gen_ai.request.temperature` - Sampling temperature
- `gen_ai.request.top_p` - Top-p parameter
- `gen_ai.usage.input_tokens` - Input token count
- `gen_ai.usage.output_tokens` - Output token count
- `gen_ai.usage.total_tokens` - Total token count
- `gen_ai.prompt` - Input messages
- `gen_ai.completion` - Output messages
- `gen_ai.messages` - Complete message events
- `gen_ai.choice` - Model choices

### Tool Span Attributes
- `gen_ai.span.kind` - "tool"
- `gen_ai.operation.name` - "execute_tool"
- `gen_ai.tool.name` - Tool name
- `gen_ai.tool.input` / `cozeloop.input` / `gen_ai.input` - Tool input
- `gen_ai.tool.output` / `cozeloop.output` / `gen_ai.output` - Tool output

### Workflow Span Attributes
- `gen_ai.span.kind` - "workflow"
- `gen_ai.operation.name` - "invocation"

## Configuration

### YAML Configuration

Add an `observability` section to your `config.yaml`:

```yaml
observability:
  opentelemetry:
    enable_global_provider: true  # Enable global OTel provider (optional)
    cozeloop:
      endpoint: "https://api.coze.cn/v1/loop/opentelemetry/v1/traces"
      api_key: "YOUR_COZE_API_KEY"
      service_name: "YOUR_COZE_SPACE_ID"
    apmplus:
      endpoint: "https://apmplus-cn-beijing.volces.com:4318"
      api_key: "YOUR_APMPLUS_API_KEY"
      service_name: "YOUR_SERVICE_NAME"
    tls:
      endpoint: "https://tls-cn-beijing.volces.com:4318/v1/traces"
      service_name: "YOUR_TLS_TOPIC"
      region: "cn-beijing"
```

### Environment Variables

All settings can be overridden via environment variables:

- `OBSERVABILITY_OPENTELEMETRY_COZELOOP_API_KEY`
- `OBSERVABILITY_OPENTELEMETRY_APMPLUS_API_KEY`
- `OBSERVABILITY_OPENTELEMETRY_ENABLE_GLOBAL_PROVIDER` (default: false)
- `VEADK_USER_ID` - Set default user ID
- `VEADK_SESSION_ID` - Set default session ID
- `VEADK_APP_NAME` - Set default app name
- `VEADK_MODEL_PROVIDER` - Set model provider
- `VEADK_CALL_TYPE` - Set call type

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
