import { start } from "./main";

const root = document.getElementById("app");
if (root) {
  start({ root }).catch((e: unknown) => {
    console.error("muster app connect failed", e);
    root.textContent = "Muster could not connect to the host. Close and reopen the view.";
  });
}
