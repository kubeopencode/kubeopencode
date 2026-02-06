# KubeOpenCode Roadmap

This document tracks planned features and improvements for KubeOpenCode.

## Post-v0.1

### Token Usage Tracking & Cost Reporting

**Status**: Deferred — waiting for upstream OpenCode support

Track per-Task token consumption (input/output/reasoning/cache tokens) and estimated cost in `TaskExecutionStatus`. Expose data via Prometheus metrics for monitoring dashboards and governance.

**Blocked on**: OpenCode currently lacks a machine-readable output for token statistics (`opencode stats` only supports ASCII table format). The cleanest implementation requires one of:
- Upstream OpenCode adding `opencode stats --format json`
- A post-run hook or summary file written by `opencode run`
- A way to retrieve `sessionID` from a completed `opencode run` without using `--format json` (which replaces human-readable logs)

Additionally, Pod mode and Server mode have different data locality — session data lives on the local filesystem in Pod mode but on the remote server in Server mode — requiring different extraction strategies.

**ADR**: [ADR 0013: Defer Token Usage Tracking](docs/adr/0013-defer-token-usage-tracking.md)
