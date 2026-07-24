# Changelog

Notable changes to `mcp-vmanomaly` are documented in this file. Release sections are also
published as the corresponding GitHub release notes.

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
