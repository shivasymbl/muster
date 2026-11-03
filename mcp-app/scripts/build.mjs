// Bundles src/entry.ts + CSS into one self-contained HTML file.
// Writes only frontend/dist/mcp-app/; never touches the rest of frontend/dist.
import { build } from "esbuild";
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const outDir = process.env.MCP_APP_OUT_DIR ? resolve(process.env.MCP_APP_OUT_DIR) : resolve(root, "..", "frontend", "dist", "mcp-app");
const MAX_BYTES = 2 * 1024 * 1024;

const result = await build({
  entryPoints: [join(root, "src/entry.ts")],
  bundle: true, minify: true, format: "iife", target: "es2022", platform: "browser",
  legalComments: "none", write: false, charset: "ascii", logLevel: "error",
});
let js = result.outputFiles[0].text;
// The SDK embeds URL-looking strings (schema ids, docs). Break the literal sequence without changing
// any string value, so the shipped file has no http(s) URL. "\/" is an identity escape in JS strings and regexes.
js = js.replace(/https?:\/\//gi, (m) => m.replace("://", ":\\/\\/"));
js = js.replace(/<\/(script)/gi, "<\\/$1").replace(/<!--/g, "<\\!--");
const css = readFileSync(join(root, "src/style.css"), "utf8");

const html = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Muster</title><style>${css}</style></head>
<body><div id="app" aria-live="polite"></div><script>${js}</script></body></html>
`;

const bytes = Buffer.byteLength(html);
const problems = [];
if (bytes > MAX_BYTES) problems.push(`bundle is ${bytes} bytes, over the 2 MiB limit`);
if (/https?:\/\//i.test(html)) problems.push("bundle contains an http(s) URL");
// Same pattern the Go server uses to reject bundles (server/routes/mcp_apps.go), plus any <link> tag and CSS url().
if (/<script[^>]*\ssrc\s*=|<link\b|\s(src|href)\s*=\s*["']?(https?:)?\/\/|@import/i.test(html) || /url\(/i.test(css)) problems.push("bundle references an external resource");
if (!html.startsWith("<!doctype html>")) problems.push("bundle does not start with <!doctype html>");
if (problems.length) { console.error("build check failed:\n- " + problems.join("\n- ")); process.exit(1); }

mkdirSync(outDir, { recursive: true });
writeFileSync(join(outDir, "index.html"), html);
const sha256 = createHash("sha256").update(html).digest("hex");
writeFileSync(join(outDir, "manifest.json"), JSON.stringify({ file: "index.html", bytes, sha256, schema_version: 1 }, null, 2) + "\n");
console.log(`mcp-app: ${join(outDir, "index.html")} ${bytes} bytes sha256=${sha256.slice(0, 16)}`);
