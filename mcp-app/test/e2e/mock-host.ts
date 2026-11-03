import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { JSDOM } from "jsdom";

const root = resolve(__dirname, "..", "..");

export function buildBundle(): string {
  const out = mkdtempSync(join(tmpdir(), "mcp-app-e2e-"));
  execFileSync("node", [join(root, "scripts", "build.mjs")], { env: { ...process.env, MCP_APP_OUT_DIR: out }, stdio: "pipe" });
  return readFileSync(join(out, "index.html"), "utf8");
}

type Msg = { jsonrpc: "2.0"; id?: number | string; method?: string; params?: any; result?: any; error?: any };
export interface MockHost {
  dom: JSDOM;
  received: Msg[];
  calls: { name: string; arguments: any }[];
  toolHandler: (name: string, args: any) => unknown | Promise<unknown>;
  send(msg: Msg): void;
  notify(method: string, params: unknown): void;
  request(method: string, params: unknown): Promise<any>;
  ready: Promise<void>;
  hostContext: Record<string, unknown>;
  displayModeResponse: "ok" | "refuse";
}

export function mockHost(html: string): MockHost {
  const received: Msg[] = [];
  const pending = new Map<number | string, (m: Msg) => void>();
  let nextId = 1000;
  let resolveReady!: () => void;
  const host = {
    received, calls: [] as { name: string; arguments: any }[],
    toolHandler: (() => { throw new Error("no handler"); }) as MockHost["toolHandler"],
    hostContext: {
      theme: "dark", displayMode: "inline", availableDisplayModes: ["inline", "fullscreen"], timeZone: "UTC",
      safeAreaInsets: { top: 1, right: 2, bottom: 3, left: 4 },
      styles: { variables: { "--color-background-primary": "#101010", "--color-text-primary": "#eeeeee" } },
    },
    displayModeResponse: "ok" as "ok" | "refuse",
    ready: new Promise<void>((r) => { resolveReady = r; }),
  } as unknown as MockHost;

  const fakeParent = {
    postMessage(data: Msg) {
      received.push(data);
      void handle(data);
    },
  };
  const dom = new JSDOM(html, {
    runScripts: "dangerously", pretendToBeVisual: true, url: "https://app.invalid/",
    beforeParse(win: any) {
      // jsdom has no ResizeObserver; real hosts do. The SDK uses it for auto-resize.
      (win as any).ResizeObserver = class { observe() {} unobserve() {} disconnect() {} };
      Object.defineProperty(win, "parent", { value: fakeParent, configurable: true }); },
  });
  host.dom = dom;

  host.send = (msg) => {
    const ev = new dom.window.Event("message");
    Object.defineProperty(ev, "data", { value: msg });
    Object.defineProperty(ev, "source", { value: fakeParent });
    dom.window.dispatchEvent(ev);
  };
  host.notify = (method, params) => host.send({ jsonrpc: "2.0", method, params });
  host.request = (method, params) => new Promise((res) => {
    const id = nextId++;
    pending.set(id, res);
    host.send({ jsonrpc: "2.0", id, method, params });
  });

  async function handle(m: Msg): Promise<void> {
    if (m.id !== undefined && m.method === undefined) { pending.get(m.id)?.(m); pending.delete(m.id); return; }
    if (m.method === "ui/initialize") {
      host.send({ jsonrpc: "2.0", id: m.id, result: {
        protocolVersion: m.params.protocolVersion, hostInfo: { name: "mock-host", version: "1" },
        hostCapabilities: { serverTools: {}, message: {}, openLinks: {} }, hostContext: host.hostContext,
      } });
    } else if (m.method === "ui/notifications/initialized") {
      resolveReady();
    } else if (m.method === "tools/call") {
      host.calls.push({ name: m.params.name, arguments: m.params.arguments });
      try {
        const result = await host.toolHandler(m.params.name, m.params.arguments);
        host.send({ jsonrpc: "2.0", id: m.id, result });
      } catch (e) {
        host.send({ jsonrpc: "2.0", id: m.id, error: { code: -32000, message: (e as Error).message } });
      }
    } else if (m.method === "ui/request-display-mode") {
      if (host.displayModeResponse === "refuse") host.send({ jsonrpc: "2.0", id: m.id, error: { code: -32000, message: "refused" } });
      else host.send({ jsonrpc: "2.0", id: m.id, result: { mode: m.params.mode } });
    } else if (m.id !== undefined) {
      host.send({ jsonrpc: "2.0", id: m.id, result: {} });
    }
  }
  return host;
}

export const tick = (ms = 20): Promise<void> => new Promise((r) => setTimeout(r, ms));
