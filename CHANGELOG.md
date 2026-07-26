# Changelog

Notable changes to `mcp-vmanomaly` are documented in this file. Release sections are also
published as the corresponding GitHub release notes.

## [Unreleased]

### Security

- Added a positive tool allowlist and enforced both allow- and denylists during tool
  discovery and execution, preventing a client from directly invoking a hidden tool.
- Added file-based bearer-token loading for container and orchestrator secret mounts.
- Removed tool arguments, results, raw errors, client metadata, and resource URIs from logs
  and metric labels where they could expose sensitive data or create unbounded cardinality.
- Require cryptographically signed release tags and publish GitHub build-provenance
  attestations for release archives.

### Improvements

- Defer documentation indexing until the first search and exclude non-served documentation
  images from binaries.
- Strip release and container binaries, remove local build paths, and run source-built
  containers as an unprivileged user.
- Added `--version` and non-zero exit codes for invalid configuration and runtime failures.
- Fixed resource capability advertisement when resources are disabled; documentation search
  remains available independently.

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

[v0.3.0]: https://github.com/VictoriaMetrics/mcp-vmanomaly/compare/v0.2.7...v0.3.0
