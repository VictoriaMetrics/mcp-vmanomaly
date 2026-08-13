# Changelog

Notable changes to `mcp-vmanomaly` are documented in this file. Release sections are also
published as the corresponding GitHub release notes.

## [v0.3.2] - 2026-08-13

### Improvements

- Refreshed embedded documentation for [`vmanomaly` v1.30.2](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1302).
- Updated configuration guidance for query-level business policies and bounded `reader.workers` concurrency.

### Compatibility

- Existing MCP tools remain compatible with `vmanomaly` v1.28.3 and newer; this release has no breaking MCP tool changes.

## [v0.3.1] - 2026-08-06

### Highlights

- Hardened MCP tool discovery, execution, logging, secret loading, containers, and release
  artifacts for safer production deployments.
- Refreshed the embedded documentation for
  [`vmanomaly` v1.30.1](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1301)
  and aligned recommendations with its online-first model guidance.
- Improved startup and failure behavior with version reporting, accurate exit codes, and
  consistent resource capability advertisement.

### Security

- Added a positive tool allowlist and enforced both allow- and denylists during tool
  discovery and execution, preventing a client from directly invoking a hidden tool.
- Added file-based bearer-token loading for container and orchestrator secret mounts.
- Removed tool arguments, results, raw errors, client metadata, and resource URIs from logs
  and metric labels where they could expose sensitive data or create unbounded cardinality.
- Required GitHub-verified cryptographically signed release tags and published GitHub
  build-provenance attestations for release archives.

### Improvements

- Refreshed the embedded documentation for
  [`vmanomaly` v1.30.1](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1301).
- Updated model-selection guidance to prefer online models and present Temporal Envelope as the
  migration target for supported offline models planned for future deprecation.
- Deferred documentation indexing until the first search and excluded non-served documentation
  images from binaries.
- Stripped release and container binaries, removed local build paths, and ran source-built
  containers as an unprivileged user.
- Added `--version` and non-zero exit codes for invalid configuration and runtime failures.
- Fixed resource capability advertisement when resources are disabled; documentation search
  remains available independently.

### Compatibility

- Existing MCP tools remain compatible with `vmanomaly` v1.28.3 and newer.
- Time-series characteristics and shared autotune require `vmanomaly` v1.30.0 or newer.
- This release contains no breaking MCP tool changes.

## [v0.3.0] - 2026-07-24

### Highlights

- Added aggregate time-series characteristics for inspecting real queries and making
  data-driven model recommendations.
- Added asynchronous shared autotune tools to create, inspect, and cancel tuning tasks for
  one validated configuration across a bounded sample of time series.
- Improved the guided workflow to reuse a user-provided query, request one when missing,
  prefer efficient online models, and validate the final configuration before applying it.

### Improvements

- Aligned model guidance and embedded documentation with `vmanomaly` v1.30, including
  Temporal Envelope, causal online validation, frozen autotune parameters, and the latest
  query-server contracts.
- Added support for documented multivariate models in shared autotune outside VMUI while
  keeping them intentionally hidden from UI discovery and schema flows.
- Improved query sampling, task response handling, validation bounds, compatibility
  reporting, and integration coverage.

### Compatibility

- Existing MCP tools remain compatible with `vmanomaly` v1.28.3 and newer.
- Time-series characteristics and shared autotune require `vmanomaly` v1.30.0 or newer.
- This release contains no breaking MCP tool changes.

[v0.3.2]: https://github.com/VictoriaMetrics/mcp-vmanomaly/compare/v0.3.1...v0.3.2
[v0.3.1]: https://github.com/VictoriaMetrics/mcp-vmanomaly/compare/v0.3.0...v0.3.1
[v0.3.0]: https://github.com/VictoriaMetrics/mcp-vmanomaly/compare/v0.2.7...v0.3.0
