# mcp-app

Read-only Muster MCP App: one self-contained HTML file served as the `ui://` resource for `muster_open_view`.
It renders `_meta.muster` (schema_version 1, see `server/services/mcp/snapshot.go`) for three workflows: pick-a-time, answer, run-a-group.

- Needs Node >= 20. `npm ci && npm run build` writes `frontend/dist/mcp-app/index.html` and `manifest.json` only; the rest of `frontend/dist` is untouched. `MCP_APP_OUT_DIR` overrides the output directory.
- `npm run typecheck`, `test:unit`, `test:perf`, `test:e2e`. Each fails if it runs zero tests. e2e builds the bundle and drives it in jsdom against a mock host (no network).
- No credentials in the iframe: reads go through the host bridge `callServerTool("muster_open_view")`. No write tools, no polling, no chat box. Names and event text are always text nodes.
- Weekly (reference-week) times are shown as UTC weekday and clock in the 2018 reference week, never as dates in the current week.
