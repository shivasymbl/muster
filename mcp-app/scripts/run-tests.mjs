// Runs vitest on one directory and fails if it discovers or runs zero tests.
import { spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const dir = process.argv[2];
if (!dir) { console.error("usage: run-tests.mjs <test-dir>"); process.exit(2); }
const tmp = mkdtempSync(join(tmpdir(), "mcp-app-test-"));
const report = join(tmp, "report.json");
const r = spawnSync(join(root, "node_modules", ".bin", "vitest"),
  ["run", dir, "--reporter=default", "--reporter=json", `--outputFile.json=${report}`], { cwd: root, stdio: "inherit" });
let code = r.status ?? 1;
try {
  if (!existsSync(report)) throw new Error("no test report produced");
  const j = JSON.parse(readFileSync(report, "utf8"));
  if (!(j.numTotalTests > 0) || !(j.numPassedTests > 0)) { console.error(`FAIL: ${dir} ran ${j.numTotalTests} tests (${j.numPassedTests} passed); zero tests is a failure`); code = 1; }
  else console.log(`${dir}: ${j.numPassedTests}/${j.numTotalTests} tests passed`);
} catch (e) { console.error("FAIL:", e.message); code = 1; }
rmSync(tmp, { recursive: true, force: true });
process.exit(code);
