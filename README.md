# MCP Server for vmanomaly

[![Latest Release](https://img.shields.io/github/v/release/VictoriaMetrics/mcp-vmanomaly?sort=semver&label=&logo=github&labelColor=gray&color=gray)](https://github.com/VictoriaMetrics/mcp-vmanomaly/releases)
![License](https://img.shields.io/github/license/VictoriaMetrics/mcp-vmanomaly?labelColor=green&label=&link=https%3A%2F%2Fgithub.com%2FVictoriaMetrics%2Fmcp-vmanomaly%2Fblob%2Fmain%2FLICENSE)
![Slack](https://img.shields.io/badge/Join-4A154B?logo=slack&link=https%3A%2F%2Fslack.victoriametrics.com)
![X](https://img.shields.io/twitter/follow/VictoriaMetrics?style=flat&label=Follow&color=black&logo=x&labelColor=black&link=https%3A%2F%2Fx.com%2FVictoriaMetrics)

The implementation of [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server for [`vmanomaly`](https://docs.victoriametrics.com/anomaly-detection/) - VictoriaMetrics Anomaly Detection product.

This provides seamless integration with `vmanomaly` REST API and [documentation](https://docs.victoriametrics.com/anomaly-detection/) for AI-assisted anomaly detection, model management, and observability insights.

## Features

This MCP server enables AI assistants like Claude to interact with `vmanomaly` for:

- **Health Monitoring**: Check `vmanomaly` server health and build information
- **Model Management**: Discover UI-compatible models and validate univariate or multivariate configurations
- **Data-Driven Recommendations**: Profile sampled time series and run shared autotune suggestions for one production-ready model config across many returned series
- **Configuration Generation**: Generate complete `vmanomaly` YAML configurations
- **Alert Rule Generation**: Generate [`vmalert`](https://docs.victoriametrics.com/victoriametrics/vmalert/) [alerting rules](https://docs.victoriametrics.com/victoriametrics/vmalert/#alerting-rules) based on [anomaly score metrics](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score) to simplify alerting setup
- **Documentation Search**: Full-text search across embedded `vmanomaly` documentation with fuzzy matching

The MCP server contains embedded up-to-date `vmanomaly` documentation and is able to search it without online access.

> The quality of the MCP Server and its responses depends very much on the capabilities of your client and the quality of the model you are using.

## Requirements

- [`vmanomaly`](https://docs.victoriametrics.com/anomaly-detection/) instance with REST API access:
  - version [1.28.3](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1283)+ for the core MCP toolset
  - version [1.30.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1300)+ for time-series characteristics and task-based shared autotune
- Go 1.24 or higher (if building from source)

## Installation

### Go

```bash
go install github.com/VictoriaMetrics/mcp-vmanomaly/cmd/mcp-vmanomaly@vX.Y.Z
```

Replace `vX.Y.Z` with the exact release you have reviewed.

### Binaries

Download the latest release from [Releases](https://github.com/VictoriaMetrics/mcp-vmanomaly/releases) page and put it to your PATH.

Example for Linux x86_64 (other architectures and platforms are also available). Select an
explicit release rather than a mutable `latest` URL, verify its checksum, and then verify its
GitHub build-provenance attestation:

```bash
version=vX.Y.Z
archive=mcp-vmanomaly_Linux_x86_64.tar.gz
curl -fLO "https://github.com/VictoriaMetrics/mcp-vmanomaly/releases/download/${version}/${archive}"
curl -fLO "https://github.com/VictoriaMetrics/mcp-vmanomaly/releases/download/${version}/checksums.txt"
grep "  ${archive}$" checksums.txt | sha256sum --check -
gh attestation verify "${archive}" --repo VictoriaMetrics/mcp-vmanomaly
tar axvf "${archive}"
./mcp-vmanomaly --version
```

Build-provenance attestations are available for releases produced by the hardened release
workflow. Release tags must be annotated, cryptographically signed, and marked as verified by
GitHub before that workflow publishes artifacts.

### Docker

You can run `vmanomaly` MCP Server using Docker.

This is the easiest way to get started without needing to install Go or build from source.

```bash
docker run -d --name mcp-vmanomaly \
  --add-host=host.docker.internal:host-gateway \
  -e VMANOMALY_ENDPOINT=http://host.docker.internal:8490 \
  -e MCP_SERVER_MODE=http \
  -e MCP_LISTEN_ADDR=:8080 \
  -p 127.0.0.1:8080:8080 \
  ghcr.io/victoriametrics/mcp-vmanomaly:vX.Y.Z
```

Replace `vX.Y.Z` and the environment variables with your own parameters. When both services
run in Docker, prefer a private Docker network and use the vmanomaly service name as the endpoint.

Note that the `MCP_SERVER_MODE=http` flag is used to enable Streamable HTTP mode.
More details about server modes can be found in the [Configuration](#configuration) section.

See available docker images in [github registry](https://github.com/VictoriaMetrics/mcp-vmanomaly/pkgs/container/mcp-vmanomaly).

Also see [Using Docker instead of binary](#using-docker-instead-of-binary) section for more details about using Docker with MCP server with clients in stdio mode.

### Source Code

For building binary from source code you can use the following approach:

- Clone repo:

  ```bash
  git clone https://github.com/VictoriaMetrics/mcp-vmanomaly.git
  cd mcp-vmanomaly
  ```

- Build binary from cloned source code:

  ```bash
  make build
  # after that you can find binary mcp-vmanomaly and copy this file to your PATH or run inplace
  ```

- Build image from cloned source code:

  ```bash
  docker build -t mcp-vmanomaly .
  # after that you can use docker image mcp-vmanomaly for running or pushing
  ```

  For local UI/Copilot testing from the vmanomaly repository, build with a local tag:

  ```bash
  docker build -t mcp-vmanomaly:local .
  ```

  Then run the vmanomaly repository helper with:

  ```bash
  MCP_VMANOMALY_IMAGE=mcp-vmanomaly:local bin/run-mcp-http.sh
  ```

## Configuration

MCP Server for vmanomaly is configured via environment variables:

| Variable                 | Description                                                                                             | Required | Default          | Allowed values         |
|--------------------------|---------------------------------------------------------------------------------------------------------|----------|------------------|------------------------|
| `VMANOMALY_ENDPOINT`     | vmanomaly server endpoint URL (e.g., http://localhost:8490)                                             | Yes      | -                | -                      |
| `VMANOMALY_BEARER_TOKEN` | Bearer token for authenticating with vmanomaly API (mutually exclusive with the token file)           | No       | -                | -                      |
| `VMANOMALY_BEARER_TOKEN_FILE` | Path to a bearer-token file, suitable for mounted container/orchestrator secrets                  | No       | -                | -                      |
| `VMANOMALY_HEADERS`      | Custom HTTP headers for requests (comma-separated key=value pairs, e.g., X-Custom=value1,X-Auth=value2) | No       | -                | -                      |
| `VMANOMALY_REQUEST_TIMEOUT` | HTTP timeout for calls from MCP to vmanomaly, e.g. `60s`                                             | No       | `30s`            | -                      |
| `MCP_SERVER_MODE`        | Server operation mode. See [Modes](#modes) for details.                                                 | No       | `stdio`          | `stdio`, `http`, `sse` |
| `MCP_LISTEN_ADDR`        | Address for HTTP server to listen on                                                                    | No       | `localhost:8080` | -                      |
| `MCP_ENABLED_TOOLS`      | Positive comma-separated tool allowlist; empty enables all registered tools                            | No       | -                | -                      |
| `MCP_DISABLED_TOOLS`     | Comma-separated tool denylist; takes precedence over the allowlist                                     | No       | -                | -                      |
| `MCP_DISABLE_RESOURCES`  | Disable all resources (documentation search will continue to work)                                      | No       | `false`          | `false`, `true`        |
| `MCP_HEARTBEAT_INTERVAL` | Heartbeat interval for streamable-http protocol (keeps connection alive through network infrastructure) | No       | `30s`            | -                      |
| `MCP_LOG_LEVEL`          | Log level: `debug` (verbose), `info` (default), `warn`, or `error`                                      | No       | `info`           | -                      |
| `MCP_LOG_FILE`           | Log file path (empty = stderr)                                                                          | No       | `stderr`         | -                      |

### Modes

MCP Server supports the following modes of operation (transports):

- `stdio` - Standard input/output mode, where the server reads commands from standard input and writes responses to standard output. This is the default mode and is suitable for local servers.
- `http` - Streamable HTTP. Server will expose the `/mcp` endpoint for HTTP connections.
- `sse` - Server-Sent Events. Server will expose the `/sse` and `/message` endpoints for SSE connections.

> [!NOTE]
> The `sse` transport mode was officially deprecated from MCP
> Specification [(version 2025-03-26)](https://modelcontextprotocol.io/specification/2025-03-26/changelog#major-changes)
> and was replaced by Streamable HTTP transport (`http` mode).
> In future releases its support can be deprecated, use Streamable HTTP transport if your client supports it.

More info about transports you can find in MCP docs:

- [Core concepts → Transports](https://modelcontextprotocol.io/docs/concepts/transports)
- [Specifications → Transports](https://modelcontextprotocol.io/specification/2025-03-26/basic/transports)

### Configuration examples

```bash
# Basic configuration
export VMANOMALY_ENDPOINT="http://localhost:8490"

# With authentication
export VMANOMALY_ENDPOINT="http://localhost:8490"
export VMANOMALY_BEARER_TOKEN="your-token"

# Or load the token from a mounted secret file
export VMANOMALY_BEARER_TOKEN_FILE="/run/secrets/vmanomaly-token"

# With custom headers (e.g., behind a reverse proxy)
export VMANOMALY_HEADERS="X-Custom-Header=value1,X-Another=value2"

# Expose only the tools required by this deployment. A denylist can further
# narrow this set and always takes precedence.
export MCP_ENABLED_TOOLS="vmanomaly_health_check,vmanomaly_search_docs"
export MCP_DISABLED_TOOLS="vmanomaly_get_metrics"

# Server mode
export MCP_SERVER_MODE="http"
export MCP_LISTEN_ADDR="0.0.0.0:8080"

# Logging
export MCP_LOG_LEVEL="debug"
export MCP_LOG_FILE="/tmp/mcp-vmanomaly.log"
```

## Endpoints

In HTTP and SSE modes the MCP server provides the following endpoints:

| Endpoint             | Description                                                                                      |
|----------------------|--------------------------------------------------------------------------------------------------|
| `/mcp`               | HTTP endpoint for streaming messages in HTTP mode (for MCP clients that support Streamable HTTP) |
| `/metrics`           | Metrics in Prometheus format for monitoring the MCP server                                       |
| `/health/liveness`   | Liveness check endpoint to ensure the server is running                                          |
| `/health/readiness`  | Readiness check endpoint to ensure the server is ready to accept requests                        |
| `/sse` + `/message`  | Endpoints for messages in SSE mode (for MCP clients that support SSE)                            |

## Security

Treat an MCP client as an operator of every enabled tool. The server forwards requests to
`vmanomaly` with the process-wide bearer token and headers configured at startup; it does not add
an independent user identity or authorization boundary.

Use one of these routing models while preserving the invariant that each tool call reaches only
the caller's trusted-domain vmanomaly installation:

- A local per-user `stdio` process may use that user's token as its configured upstream token.
- A remote MCP instance dedicated to one trusted domain may use a domain-scoped service token.
- A shared remote MCP requires per-request forwarding of a verified user token so the gateway can
  route each call to the correct trusted domain. The current process-wide token configuration does
  not implement this pass-through mode; do not place multiple untrusted domains behind one static
  MCP credential.

- Prefer `stdio` for a local, single-user integration. It has no network listener and inherits
  access control from the process that launches it.
- HTTP and SSE transports do not provide built-in client authentication. Keep the default
  loopback bind where possible. If remote access is required, place the server behind an
  authenticated TLS reverse proxy such as `vmauth`, restrict the network path, and do not expose
  `/mcp`, `/sse`, or `/message` directly to an untrusted network.
- Keep `/metrics` on an internal monitoring network or protect it at the proxy; health endpoints
  can be exposed only as required by the deployment platform.
- Give the configured vmanomaly credential the least privilege and trusted-domain scope available.
  Prefer `VMANOMALY_BEARER_TOKEN_FILE` for mounted secrets; never put tokens in command-line
  arguments, image layers, or committed client configuration.
- Treat `VMANOMALY_HEADERS` as trusted operator configuration. Tools that set
  `pass_auth_headers=true` can ask vmanomaly to forward authorization to a datasource, so permit
  that only for approved datasource origins and enforce an outbound network policy.
- Use `MCP_ENABLED_TOOLS` as a deployment allowlist. Both the allowlist and denylist are enforced
  for discovery and direct invocation, so hidden tools cannot be called by name. An empty
  allowlist retains backward compatibility by enabling every registered tool.
- `MCP_DISABLE_RESOURCES=true` hides resource discovery and reads. The documentation-search tool
  remains independent and can be separately disabled with the tool policy.
- Logs and metrics intentionally omit tool arguments/results, raw errors, client metadata, and
  resource URIs. Treat MCP responses and downstream vmanomaly logs as sensitive nevertheless.

These controls reduce the MCP server's exposure but do not create tenant isolation. Treat one
logical vmanomaly installation, including its replicas or shards, as one trusted domain. Route
mutually untrusted domains to separate installations through `vmauth` or another authenticated
gateway. Users inside one trusted domain share its task and resource boundary.

Report suspected vulnerabilities using the private process in [SECURITY.md](SECURITY.md).

## Setup in clients

### Cursor

Go to: `Settings` → `Cursor Settings` → `MCP` → `Add new global MCP server` and paste the following configuration into your Cursor `~/.cursor/mcp.json` file:

```json
{
  "mcpServers": {
    "vmanomaly": {
      "command": "/path/to/mcp-vmanomaly",
      "env": {
        "VMANOMALY_ENDPOINT": "http://localhost:8490",
        "VMANOMALY_BEARER_TOKEN": "<YOUR_TOKEN>",
        "VMANOMALY_HEADERS": "X-Custom=value1,X-Auth=value2"
      }
    }
  }
}
```

See [Cursor MCP docs](https://docs.cursor.com/context/model-context-protocol) for more info.

### Claude Desktop

Add this to your Claude Desktop `claude_desktop_config.json` file (you can find it if open `Settings` → `Developer` → `Edit config`):

```json
{
  "mcpServers": {
    "vmanomaly": {
      "command": "/path/to/mcp-vmanomaly",
      "env": {
        "VMANOMALY_ENDPOINT": "http://localhost:8490",
        "VMANOMALY_BEARER_TOKEN": "<YOUR_TOKEN>",
        "VMANOMALY_HEADERS": "X-Custom=value1,X-Auth=value2"
      }
    }
  }
}
```

See [Claude Desktop MCP docs](https://modelcontextprotocol.io/quickstart/user) for more info.

### Claude Code

Run the command:

```sh
claude mcp add vmanomaly -- /path/to/mcp-vmanomaly \
  -e VMANOMALY_ENDPOINT=http://localhost:8490 \
  -e VMANOMALY_BEARER_TOKEN=<YOUR_TOKEN> \
  -e VMANOMALY_HEADERS="X-Custom=value1,X-Auth=value2"
```

See [Claude Code MCP docs](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/tutorials#set-up-model-context-protocol-mcp) for more info.

### Visual Studio Code

Add this to your VS Code MCP config file:

```json
{
  "servers": {
    "vmanomaly": {
      "type": "stdio",
      "command": "/path/to/mcp-vmanomaly",
      "env": {
        "VMANOMALY_ENDPOINT": "http://localhost:8490",
        "VMANOMALY_BEARER_TOKEN": "<YOUR_TOKEN>",
        "VMANOMALY_HEADERS": "X-Custom=value1,X-Auth=value2"
      }
    }
  }
}
```

See [VS Code MCP docs](https://code.visualstudio.com/docs/copilot/chat/mcp-servers) for more info.

### Zed

Add the following to your Zed config file:

```json
  "context_servers": {
    "vmanomaly": {
      "command": {
        "path": "/path/to/mcp-vmanomaly",
        "args": [],
        "env": {
          "VMANOMALY_ENDPOINT": "http://localhost:8490",
          "VMANOMALY_BEARER_TOKEN": "<YOUR_TOKEN>",
          "VMANOMALY_HEADERS": "X-Custom=value1,X-Auth=value2"
        }
      },
      "settings": {}
    }
  }
```

See [Zed MCP docs](https://zed.dev/docs/ai/mcp) for more info.

### JetBrains IDEs

- Open `Settings` → `Tools` → `AI Assistant` → `Model Context Protocol (MCP)`.
- Click `Add (+)`
- Select `As JSON`
- Put the following to the input field:

```json
{
  "mcpServers": {
    "vmanomaly": {
      "command": "/path/to/mcp-vmanomaly",
      "env": {
        "VMANOMALY_ENDPOINT": "http://localhost:8490",
        "VMANOMALY_BEARER_TOKEN": "<YOUR_TOKEN>",
        "VMANOMALY_HEADERS": "X-Custom=value1,X-Auth=value2"
      }
    }
  }
}
```

### Windsurf

Add the following to your Windsurf MCP config file:

```json
{
  "mcpServers": {
    "vmanomaly": {
      "command": "/path/to/mcp-vmanomaly",
      "env": {
        "VMANOMALY_ENDPOINT": "http://localhost:8490",
        "VMANOMALY_BEARER_TOKEN": "<YOUR_TOKEN>",
        "VMANOMALY_HEADERS": "X-Custom=value1,X-Auth=value2"
      }
    }
  }
}
```

See [Windsurf MCP docs](https://docs.windsurf.com/windsurf/mcp) for more info.

### Using Docker instead of binary

You can run vmanomaly MCP server using Docker instead of local binary.

You should replace run command in configuration examples above in the following way:

```json
{
  "mcpServers": {
    "vmanomaly": {
      "command": "docker",
      "args": [
        "run",
        "-i", "--rm",
        "-e", "VMANOMALY_ENDPOINT",
        "-e", "VMANOMALY_BEARER_TOKEN",
        "-e", "VMANOMALY_HEADERS",
        "ghcr.io/victoriametrics/mcp-vmanomaly"
      ],
      "env": {
        "VMANOMALY_ENDPOINT": "http://localhost:8490",
        "VMANOMALY_BEARER_TOKEN": "<YOUR_TOKEN>",
        "VMANOMALY_HEADERS": "X-Custom=value1,X-Auth=value2"
      }
    }
  }
}
```

## Usage

After [installing](#installation) and [configuring](#setup-in-clients) the MCP server, you can start using it with your favorite MCP client.

You can start dialog with AI assistant from the phrase:

```
Use MCP vmanomaly in the following answers
```

But it's not required, you can just start asking questions and the assistant will automatically use the tools and documentation to provide you with the best answers.

### Toolset

MCP vmanomaly provides tools organized into categories:

#### Health & Info (4 tools)

| Tool                           | Description                                             |
|--------------------------------|---------------------------------------------------------|
| `vmanomaly_health_check`       | Check vmanomaly server health status                    |
| `vmanomaly_get_buildinfo`      | Get build information (version, build time, Go version) |
| `vmanomaly_get_server_queries` | Get configured server query aliases and expressions     |
| `vmanomaly_get_metrics`        | Get vmanomaly server metrics in Prometheus format       |

#### Model Configuration (4 tools)

| Tool                              | Description                                             |
|-----------------------------------|---------------------------------------------------------|
| `vmanomaly_list_models`           | List models exposed to VMUI and other UI-oriented flows |
| `vmanomaly_get_server_models`     | Get configured server models and their query attachments |
| `vmanomaly_get_model_schema`      | Get JSON schema for a specific model type               |
| `vmanomaly_validate_model_config` | Validate model configuration before using it            |

#### Configuration (1 tool)

| Tool                        | Description                                    |
|-----------------------------|------------------------------------------------|
| `vmanomaly_validate_config` | Validate complete vmanomaly YAML configuration |

#### Documentation (1 tool)

| Tool                      | Description                                                         |
|---------------------------|---------------------------------------------------------------------|
| `vmanomaly_search_docs`   | Full-text search across vmanomaly documentation with fuzzy matching |

#### Compatibility (1 tool)

| Tool                            | Description                                                 |
|---------------------------------|-------------------------------------------------------------|
| `vmanomaly_check_compatibility` | Check if persisted state is compatible with runtime version |

#### Alerting (1 tool)

| Tool                              | Description                                              |
|-----------------------------------|----------------------------------------------------------|
| `vmanomaly_generate_alert_rule`   | Generate VMAlert rule YAML for anomaly score alerting    |

#### Analysis & Autotune (4 tools)

| Tool                                   | Description                                                                  |
|----------------------------------------|------------------------------------------------------------------------------|
| `vmanomaly_timeseries_characteristics` | Profile sampled query results for trends, seasonalities, spikiness, and gaps |
| `vmanomaly_create_autotune_task`       | Start tuning one requested model class on sampled series                    |
| `vmanomaly_get_autotune_task`          | Poll autotune progress and retrieve a completed recommendation              |
| `vmanomaly_cancel_autotune_task`       | Request cooperative cancellation of an autotune task                        |

`vmanomaly_create_autotune_task` accepts `optimization_n_trials`, `optimization_timeout`, and advanced
`optimization_params` to bound Optuna work. The MCP tool uses interactive defaults of 32 trials and
8 seconds when no optimization budget is provided, and a conservative anomaly fraction of 0.02 when
`anomaly_percentage` is omitted; the vmanomaly API defaults are larger. Poll
`vmanomaly_get_autotune_task` until `status` is `done`, then use the recommendation under
`result_data`. Treat `error` and `canceled` as terminal statuses.

The list/schema endpoints intentionally expose only models supported by VMUI. Outside VMUI,
documented multivariate aliases such as `temporal_envelope_multivariate` can still be shared-autotuned
and validated as complete model configurations. Do not recommend multivariate models in VMUI.

For new configurations, prefer online models. Prophet, Holt-Winters, and Isolation Forest remain
supported for existing deployments but are planned for future deprecation; use Temporal Envelope as
the corresponding univariate or multivariate migration target.

### Dialog example

This example shows data-driven model selection rather than choosing a model from the metric name. The YAML is a
complete deployment-config excerpt: VMUI suggestion cards continue to carry business-policy fields in the model
configuration because the UI query editor currently exposes only the query expression and language.

- > **You**: Detect sustained CPU spikes for
  > `avg by (instance) (1 - rate(node_cpu_seconds_total{mode="idle"}[5m]))` at a `5m` step.
- > **Assistant**: *I profiled the exact query first. The sampled series contain a trend and recurring
  > hour-of-day/day-of-week structure, so `temporal_envelope` is the best online starting point. If
  > the profile had no meaningful trend or seasonality, I would prefer `mad_online` for robust data,
  > or `zscore_online` only for a stable light-tailed distribution.*
- > **Assistant**: *I verified the model and schema, then started shared autotune with the same `5m`
  > step, `anomaly_percentage=0.02`, and causal `exact=true` validation. After sequentially polling
  > the task to completion, I validated the returned model configuration before presenting it.*
  >
  > ```yaml
  > reader:
  >   workers: 0
  >   queries:
  >     cpu_non_idle:
  >       expr: avg by (instance) (1 - rate(node_cpu_seconds_total{mode="idle"}[5m]))
  >       detection_direction: above_expected
  >       min_rel_dev_from_expected: [0, 15]
  >
  > models:
  >   cpu_envelope:
  >     class: temporal_envelope
  >     queries: [cpu_non_idle]
  >     schedulers: [cpu_5m]
  >     seasonalities: [hod_smooth, dow_smooth]
  > ```
  >
  > *The exact tuned parameters depend on the sampled data; the final model spec is always validated
  > against the running vmanomaly instance.*

During this dialog, the assistant used the following tools:

- `vmanomaly_timeseries_characteristics` to measure the sampled data profile
- `vmanomaly_list_models` and `vmanomaly_get_model_schema` to verify the UI-compatible model
- `vmanomaly_create_autotune_task` and `vmanomaly_get_autotune_task` to tune shared parameters
- `vmanomaly_validate_model_config` to validate the tuned model
- `vmanomaly_validate_config` to validate the configuration

## Monitoring

In [HTTP and SSE modes](#modes) the MCP Server provides metrics in Prometheus format at the `/metrics` endpoint.

**Tracked operations**:

- `mcp_vmanomaly_initialize_total` - Client connections
- `mcp_vmanomaly_call_tool_total{name,is_error}` - Tool calls with success/error tracking
- `mcp_vmanomaly_read_resource_total` - Documentation resource reads
- `mcp_vmanomaly_list_*_total` - List operations (tools, resources, prompts)
- `mcp_vmanomaly_error_total{method,error_class}` - Errors by bounded, non-sensitive class

**Example**:

```bash
# Start in HTTP mode
VMANOMALY_ENDPOINT="http://localhost:8490" MCP_SERVER_MODE=http ./bin/mcp-vmanomaly

# Query metrics
curl http://localhost:8080/metrics
```

## Roadmap

- [ ] Grafana dashboard for MCP server monitoring
- [ ] Add API compatibility matrix to gracefully handle version differences between MCP client and vmanomaly server (API is evolving, features may be unavailable)

## Disclaimer

AI services and agents along with MCP servers like this cannot guarantee the accuracy, completeness and reliability of results.
You should double check the results obtained with AI.

The quality of the MCP Server and its responses depends very much on the capabilities of your client and the quality of the model you are using.

## Contributing

Contributions to the MCP vmanomaly project are welcome!

Please feel free to submit issues, feature requests, or pull requests.

## Related Projects

- [vmanomaly](https://docs.victoriametrics.com/anomaly-detection/) - VictoriaMetrics anomaly detection
- [VictoriaMetrics](https://victoriametrics.com/) - Time series database
- [mcp-victoriametrics](https://github.com/VictoriaMetrics/mcp-victoriametrics) - MCP server for VictoriaMetrics
- [Model Context Protocol](https://modelcontextprotocol.io/) - MCP specification

## Support

For vmanomaly-specific questions, see the [vmanomaly documentation](https://docs.victoriametrics.com/anomaly-detection/).

For MCP server issues, please open an issue in this repository.
