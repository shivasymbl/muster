import { App, PostMessageTransport, applyDocumentTheme, applyHostStyleVariables } from "@modelcontextprotocol/ext-apps";
import { isValidTimeZone } from "./format";
import { parseToolResult } from "./parse";
import { render, type Handlers } from "./render";
import { initialState, reduce, type Action, type State } from "./state";
import type { Workflow } from "./types";

const OPENER = "muster_open_view";
const WORKFLOWS = ["pick-a-time", "answer", "run-a-group"];

export interface StartOptions {
  root: HTMLElement;
  /** Defaults to the real parent window. Tests inject a mock host window. */
  host?: Window;
}

function pickWorkflow(v: unknown): Workflow | null {
  return typeof v === "string" && WORKFLOWS.includes(v) ? (v as Workflow) : null;
}

export async function start(opts: StartOptions): Promise<{ app: App; getState: () => State }> {
  const { root } = opts;
  const host = opts.host ?? window.parent;
  const app = new App({ name: "muster-view", version: "0.1.0" }, {}, { autoResize: true });
  let state = initialState("UTC");
  let lastArgs: Record<string, unknown> | null = null;
  let closed = false;
  let inflight: AbortController | null = null;

  const dispatch = (a: Action): void => {
    if (closed && a.type !== "teardown") return;
    state = reduce(state, a);
    render(root, state, handlers);
  };

  const read = async (extra: Record<string, unknown> = {}): Promise<void> => {
    if (closed || state.ui.reading) return;
    if (!lastArgs) { dispatch({ type: "notice", message: "Nothing to refresh yet. Ask Claude to open the view again." }); return; }
    inflight?.abort();
    const ctl = new AbortController();
    inflight = ctl;
    dispatch({ type: "read_start" });
    const seq = state.readSeq;
    try {
      // Read-only opener through the host bridge. The iframe holds no credentials.
      const res = await app.callServerTool({ name: OPENER, arguments: { ...lastArgs, ...extra } }, { signal: ctl.signal });
      if (ctl.signal.aborted) return;
      dispatch({ type: "result", parsed: parseToolResult(res), seq });
    } catch (e) {
      if (ctl.signal.aborted) return;
      dispatch({ type: "read_error", message: e instanceof Error && e.message ? `Refresh failed: ${e.message}` : "Refresh failed.", seq });
    }
  };

  const handlers: Handlers = {
    select: (index) => dispatch({ type: "select", index }),
    sort: (sort) => dispatch({ type: "sort", sort }),
    view: (view) => dispatch({ type: "view", view }),
    tz: (tz) => { if (isValidTimeZone(tz)) dispatch({ type: "tz", tz }); },
    refresh: () => { void read(); },
    loadMore: (cursor) => { void read({ people_cursor: cursor }); },
    expand: () => {
      app.requestDisplayMode({ mode: "fullscreen" }).then(
        (r) => dispatch({ type: "host", displayMode: r.mode }),
        () => dispatch({ type: "notice", message: "The host did not allow a larger view. Use the list in place." }),
      );
    },
  };

  const applyContext = (ctx: ReturnType<App["getHostContext"]>): void => {
    if (!ctx) return;
    if (ctx.theme) applyDocumentTheme(ctx.theme);
    if (ctx.styles?.variables) applyHostStyleVariables(ctx.styles.variables);
    const si = ctx.safeAreaInsets;
    if (si) {
      const s = document.documentElement.style;
      s.setProperty("--safe-top", `${si.top}px`); s.setProperty("--safe-right", `${si.right}px`);
      s.setProperty("--safe-bottom", `${si.bottom}px`); s.setProperty("--safe-left", `${si.left}px`);
    }
    const modes = ctx.availableDisplayModes;
    dispatch({
      type: "host", displayMode: ctx.displayMode,
      canExpand: !!modes && modes.includes("fullscreen") && !!app.getHostCapabilities(),
    });
  };

  // All handlers are registered before connect().
  app.ontoolinput = (p) => {
    const a = p.arguments ?? {};
    lastArgs = { ...a };
    dispatch({ type: "tool_input", workflow: pickWorkflow(a.workflow), eventId: typeof a.event_id === "string" ? a.event_id : null });
  };
  // Partial input only labels the skeleton. It is never used as read arguments.
  app.ontoolinputpartial = (p) => {
    const a = p.arguments ?? {};
    dispatch({ type: "tool_input", workflow: pickWorkflow(a.workflow), eventId: typeof a.event_id === "string" ? a.event_id : null });
  };
  app.ontoolresult = (res) => { inflight?.abort(); dispatch({ type: "result", parsed: parseToolResult(res) }); };
  app.ontoolcancelled = () => { inflight?.abort(); dispatch({ type: "cancelled" }); };
  app.onhostcontextchanged = () => applyContext(app.getHostContext());
  app.onteardown = async () => {
    closed = true;
    inflight?.abort();
    state = reduce(state, { type: "teardown" });
    root.replaceChildren();
    return {};
  };
  app.onerror = () => dispatch({ type: "notice", message: "The host connection reported a problem." });

  render(root, state, handlers);
  await app.connect(new PostMessageTransport(host, host));
  applyContext(app.getHostContext());
  return { app, getState: () => state };
}
