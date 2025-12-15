# MCP Server for vmanomaly

[![Latest Release](https://img.shields.io/github/v/release/galic-vlad/mcp-vmanomaly?sort=semver&label=&logo=github&labelColor=gray&color=gray)](https://github.com/galic-vlad/mcp-vmanomaly/releases)
[![smithery badge](https://smithery.ai/badge/@galic-vlad/mcp-vmanomaly)](https://smithery.ai/server/@galic-vlad/mcp-vmanomaly)
![License](https://img.shields.io/github/license/galic-vlad/mcp-vmanomaly?labelColor=green&label=&link=https%3A%2F%2Fgithub.com%2Fgalic-vlad%2Fmcp-vmanomaly%2Fblob%2Fmain%2FLICENSE)
![Slack](https://img.shields.io/badge/Join-4A154B?logo=slack&link=https%3A%2F%2Fslack.victoriametrics.com)
![X](https://img.shields.io/twitter/follow/VictoriaMetrics?style=flat&label=Follow&color=black&logo=x&labelColor=black&link=https%3A%2F%2Fx.com%2FVictoriaMetrics)

The implementation of [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server for [vmanomaly](https://docs.victoriametrics.com/anomaly-detection/) - VictoriaMetrics' anomaly detection application.

This provides seamless integration with vmanomaly REST API and [documentation](https://docs.victoriametrics.com/anomaly-detection/) for AI-assisted anomaly detection, model management, and observability insights.

## Features

This MCP server enables AI assistants like Claude to interact with vmanomaly for:

- **Health Monitoring**: Check vmanomaly server health and build information
- **Model Management**: List, validate, and configure anomaly detection models (10+ model types: zscore, prophet, mad, holtwinters, isolation_forest, and more)
- **Configuration Generation**: Generate complete vmanomaly YAML configurations
- **Alert Rule Generation**: Generate VMAlert rules for anomaly score alerting
- **Documentation Search**: Full-text search across embedded vmanomaly documentation with fuzzy matching

The MCP server contains embedded up-to-date vmanomaly documentation and is able to search it without online access.

The quality of the MCP Server and its responses depends very much on the capabilities of your client and the quality of the model you are using.

## Requirements

- [vmanomaly](https://docs.victoriametrics.com/anomaly-detection/) instance with REST API access
- Go 1.24 or higher (if building from source)

## Installation

### Go

```bash
go install github.com/galic-vlad/mcp-vmanomaly/cmd/mcp-vmanomaly@latest
```

### Binaries

Download the latest release from [Releases](https://github.com/galic-vlad/mcp-vmanomaly/releases) page and put it to your PATH.

Example for Linux x86_64 (other architectures and platforms are also available):

```bash
latest=$(curl -s https://api.github.com/repos/galic-vlad/mcp-vmanomaly/releases/latest | grep 'tag_name' | cut -d\" -f4)
wget https://github.com/galic-vlad/mcp-vmanomaly/releases/download/$latest/mcp-vmanomaly_Linux_x86_64.tar.gz
tar axvf mcp-vmanomaly_Linux_x86_64.tar.gz
```

### Docker

You can run vmanomaly MCP Server using Docker.

This is the easiest way to get started without needing to install Go or build from source.

```bash
docker run -d --name mcp-vmanomaly \
  -e VMANOMALY_ENDPOINT=http://localhost:8490 \
  -e MCP_SERVER_MODE=http \
  -e MCP_LISTEN_ADDR=:8080 \
  -p 8080:8080 \
  ghcr.io/galic-vlad/mcp-vmanomaly
```

You should replace environment variables with your own parameters.

Note that the `MCP_SERVER_MODE=http` flag is used to enable Streamable HTTP mode.
More details about server modes can be found in the [Configuration](#configuration) section.

See available docker images in [github registry](https://github.com/galic-vlad/mcp-vmanomaly/pkgs/container/mcp-vmanomaly).

Also see [Using Docker instead of binary](#using-docker-instead-of-binary) section for more details about using Docker with MCP server with clients in stdio mode.

### Source Code

For building binary from source code you can use the following approach:

- Clone repo:

  ```bash
  git clone https://github.com/galic-vlad/mcp-vmanomaly.git
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

### Smithery

To install vmanomaly MCP Server for your client automatically via Smithery, you can use the following commands:

```bash
# Get the list of supported MCP clients
npx -y @smithery/cli list clients
#Available clients:
#  claude
#  cline
#  windsurf
#  roocode
#  witsy
#  enconvo
#  cursor
#  vscode
#  vscode-insiders
#  boltai
#  amazon-bedrock

# Install vmanomaly MCP server for your client
npx -y @smithery/cli install @galic-vlad/mcp-vmanomaly --client <YOUR-CLIENT-NAME>
# and follow the instructions
```

## Configuration

MCP Server for vmanomaly is configured via environment variables:

| Variable                 | Description                                                                                             | Required | Default          | Allowed values  |
|--------------------------|---------------------------------------------------------------------------------------------------------|----------|------------------|-----------------|
| `VMANOMALY_ENDPOINT`     | vmanomaly server endpoint URL (e.g., http://localhost:8490)                                             | Yes      | -                | -               |
| `VMANOMALY_BEARER_TOKEN` | Bearer token for authenticating with vmanomaly API                                                      | No       | -                | -               |
| `VMANOMALY_HEADERS`      | Custom HTTP headers for requests (comma-separated key=value pairs, e.g., X-Custom=value1,X-Auth=value2) | No       | -                | -               |
| `MCP_SERVER_MODE`        | Server operation mode. See [Modes](#modes) for details.                                                 | No       | `stdio`          | `stdio`, `http` |
| `MCP_LISTEN_ADDR`        | Address for HTTP server to listen on                                                                    | No       | `localhost:8080` | -               |
| `MCP_DISABLED_TOOLS`     | Comma-separated list of tools to disable                                                                | No       | -                | -               |
| `MCP_DISABLE_RESOURCES`  | Disable all resources (documentation search will continue to work)                                      | No       | `false`          | `false`, `true` |
| `MCP_HEARTBEAT_INTERVAL` | Heartbeat interval for streamable-http protocol (keeps connection alive through network infrastructure) | No       | `30s`            | -               |
| `MCP_LOG_LEVEL`          | Log level: `debug` (verbose), `info` (default), `warn`, or `error`                                      | No       | `info`           | -               |
| `MCP_LOG_FILE`           | Log file path (empty = stderr)                                                                          | No       | `stderr`         | -               |

### Modes

MCP Server supports the following modes of operation (transports):

- `stdio` - Standard input/output mode, where the server reads commands from standard input and
  writes responses to standard output. This is the default mode and is suitable for local servers.
- `http` - Streamable HTTP. Server will expose the `/mcp` endpoint for HTTP connections.

> [!NOTE]
> The `sse` transport mode was officialy deprecated from MCP
> Specification [(version 2025-03-26)](https://modelcontextprotocol.io/specification/2025-03-26/changelog#major-changes)
> and was replaced by Streamable HTTP transport (`http` mode).


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

# With custom headers (e.g., behind a reverse proxy)
export VMANOMALY_HEADERS="X-Custom-Header=value1,X-Another=value2"

# Server mode
export MCP_SERVER_MODE="http"
export MCP_LISTEN_ADDR="0.0.0.0:8080"

# Logging
export MCP_LOG_LEVEL="debug"
export MCP_LOG_FILE="/tmp/mcp-vmanomaly.log"
```

## Endpoints

In HTTP mode the MCP server provides the following endpoints:

| Endpoint             | Description                                                                                       |
|----------------------|---------------------------------------------------------------------------------------------------|
| `/mcp`               | HTTP endpoint for streaming messages in HTTP mode (for MCP clients that support Streamable HTTP) |
| `/metrics`           | Metrics in Prometheus format for monitoring the MCP server                                        |
| `/health/liveness`   | Liveness check endpoint to ensure the server is running                                           |
| `/health/readiness`  | Readiness check endpoint to ensure the server is ready to accept requests                         |

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
        "ghcr.io/galic-vlad/mcp-vmanomaly"
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

#### Health & Info (3 tools)

| Tool                       | Description                                             |
|----------------------------|---------------------------------------------------------|
| `vmanomaly_health_check`   | Check vmanomaly server health status                    |
| `vmanomaly_get_buildinfo`  | Get build information (version, build time, Go version) |
| `vmanomaly_get_metrics`    | Get vmanomaly server metrics in Prometheus format       |

#### Model Configuration (3 tools)

| Tool                              | Description                                      |
|-----------------------------------|--------------------------------------------------|
| `vmanomaly_list_models`           | List all available anomaly detection model types |
| `vmanomaly_get_model_schema`      | Get JSON schema for a specific model type        |
| `vmanomaly_validate_model_config` | Validate model configuration before using it     |

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

### Dialog example

This is an example dialog showing how AI assistant can help with vmanomaly configuration and anomaly detection:

- > **You**: I want to set up anomaly detection for CPU usage. What models are available?
- > **Assistant**: *I can help you set up anomaly detection for CPU usage. Let me check available models in vmanomaly.*
  >
  > *Your vmanomaly instance supports the following anomaly detection models:*
  > - *`zscore` - Statistical model based on z-score (standard deviations from mean)*
  > - *`prophet` - Facebook's Prophet model for time series with seasonality*
  > - *`mad` - Median Absolute Deviation model*
  > - *`holtwinters` - Holt-Winters seasonal method*
  > - *`isolation_forest_univariate` - Isolation Forest for univariate time series*
  > - *`rolling_quantile` - Rolling quantile-based detection*
  > - *`auto` - Automatic model selection*
  >
  > *For CPU usage monitoring, I'd recommend starting with `zscore` model as it's simple, fast, and works well for metrics with stable patterns. Would you like me to generate a configuration?*

- > **You**: Yes, please generate a configuration for monitoring CPU usage with zscore model
- > **Assistant**: *I'll generate a complete vmanomaly configuration for CPU usage monitoring with the zscore model:*
  >
  > ```yaml
  > scheduler:
  >   infer_every: "1m"
  >   fit_every: "1h"
  >   fit_window: "2d"
  >
  > reader:
  >   datasource_url: "http://victoriametrics:8428"
  >   queries:
  >     cpu_usage: "avg(rate(node_cpu_seconds_total{mode!='idle'}[5m])) by (instance)"
  >   step: "1m"
  >
  > models:
  >   zscore_cpu:
  >     class: "zscore"
  >     z_threshold: 2.5
  >
  > writer:
  >   datasource_url: "http://victoriametrics:8428"
  > ```
  >
  > *This configuration will:*
  > - *Run inference every minute*
  > - *Retrain the model every hour using the last 2 days of data*
  > - *Use z-score model with threshold of 2.5 standard deviations*
  > - *Monitor CPU usage across all instances*
  >
  > *Would you like me to validate this configuration or adjust any parameters?*

- > **You**: Looks good! Can you create a detection task to test it?
- > **Assistant**: *I'll create an anomaly detection task to test the configuration on your recent CPU data:*
  >
  > *Task created successfully! Here's the status:*
  > - *Task ID: `task-abc-123`*
  > - *Status: Running*
  > - *Model: zscore with z_threshold=2.5*
  > - *Time range: Last 24 hours*
  > - *Progress: Processing data...*
  >
  > *The task is analyzing your CPU usage data. You can check the results in VictoriaMetrics by querying the anomaly score metrics. Would you like me to check the task status or show you how to query the results?*

During this dialog, the assistant used the following tools:

- `vmanomaly_list_models` to get available model types
- `vmanomaly_validate_config` to validate the configuration
- `vmanomaly_create_detection_task` to start anomaly detection
- `vmanomaly_search_docs` to provide context about model parameters

## Monitoring

In [HTTP mode](#modes) the MCP Server provides metrics in Prometheus format at the `/metrics` endpoint.

**Tracked operations**:

- `mcp_vmanomaly_initialize_total` - Client connections
- `mcp_vmanomaly_call_tool_total{name,is_error}` - Tool calls with success/error tracking
- `mcp_vmanomaly_read_resource_total{uri}` - Documentation resource reads
- `mcp_vmanomaly_list_*_total` - List operations (tools, resources, prompts)
- `mcp_vmanomaly_error_total{method,error}` - Errors by method and type

**Example**:

```bash
# Start in HTTP mode
VMANOMALY_ENDPOINT="http://localhost:8490" MCP_SERVER_MODE=http ./bin/mcp-vmanomaly

# Query metrics
curl http://localhost:8080/metrics
```

## Roadmap

- [ ] Add prompts for common vmanomaly workflows (model selection, troubleshooting)
- [ ] Grafana dashboard for MCP server monitoring

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
- [mcp-victoriametrics](https://github.com/VictoriaMetrics-Community/mcp-victoriametrics) - MCP server for VictoriaMetrics
- [Model Context Protocol](https://modelcontextprotocol.io/) - MCP specification

## Support

For vmanomaly-specific questions, see the [vmanomaly documentation](https://docs.victoriametrics.com/anomaly-detection/).

For MCP server issues, please open an issue in this repository.
