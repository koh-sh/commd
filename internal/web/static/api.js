// Server calls. Every successful call returns the whole session state,
// which is applied and rendered.

import { $, h, toast } from "./dom.js";
import { token, st, setState, ui, visibleLines } from "./state.js";
import { render } from "./render.js";
import { onKeyDown } from "./keys.js";

// api calls the server and applies the returned state. When the command
// is gone, the page switches to the end screen. A conflict means the
// session moved on (e.g. from another tab): the state is reloaded.
export async function api(method, path, body) {
  let res;
  try {
    res = await fetch(path, {
      method,
      headers: { "X-Commd-Token": token, "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    showEnd("Review session ended", "commd is no longer running (it was stopped in the terminal). Nothing more can be saved.");
    return false;
  }
  if (res.status === 409) {
    toast((await res.json().catch(() => ({}))).error || "The review moved on.");
    await load();
    return false;
  }
  if (!res.ok) {
    let msg = `${res.status} ${res.statusText}`;
    try {
      msg = (await res.json()).error || msg;
    } catch {
      // keep the status text
    }
    toast(msg);
    return false;
  }
  apply(await res.json());
  return true;
}

// fileAPI calls an endpoint of the file under review.
export const fileAPI = (method, path, body) => api(method, `/api/files/${st.seq}${path}`, body);

export async function load() {
  let res;
  try {
    res = await fetch("/api/state", { headers: { "X-Commd-Token": token } });
  } catch {
    showEnd("Review session ended", "commd is no longer running. Nothing more can be saved.");
    return;
  }
  if (!res.ok) {
    showEnd("Cannot load the review", (await res.json().catch(() => ({}))).error || res.statusText);
    return;
  }
  apply(await res.json());
}

// apply takes a new server state. A new file starts with fresh view state,
// like the TUI creating a new App per file.
export function apply(state) {
  setState(state);
  if (st.phase === "done") {
    showEnd("Review finished", "commd has the result. You can close this tab.");
    return;
  }
  if (st.phase === "review" && st.seq !== ui.seq) resetFileUI();
  render();
}

export function resetFileUI() {
  const f = st.file;
  Object.assign(ui, {
    seq: st.seq,
    mode: "normal",
    sidebarOpen: false,
    cursor: f.sections.length ? f.sections[0].id : null,
    fullView: false,
    rawView: f.diff, // diffs open in the raw (diff) view, like the TUI
    lineCursor: 0,
    anchor: -1,
    query: "",
    collapsed: new Set(),
    leftRatio: 20,
    pendingG: false,
    editor: null,
    list: null,
    confirm: null,
  });
  ui.lineCursor = visibleLines()[0] ?? 0;
}

export function showEnd(title, detail) {
  if (ui.finished) return;
  ui.finished = true;
  $("#modal-root").replaceChildren();
  $("#app").replaceChildren(h("div", { class: "done" }, h("h1", {}, title), h("p", {}, detail)));
  document.title = `commd — ${title}`;
}

// send runs one request at a time; keys pressed meanwhile are queued and
// replayed afterwards.
export async function send(request) {
  if (ui.busy) return false;
  ui.busy = true;
  try {
    return await request();
  } finally {
    ui.busy = false;
    // Replay keys pressed meanwhile (the TUI never drops keys), after the
    // caller has applied the response.
    setTimeout(replayKeys, 0);
  }
}

// queuedKeys holds keys pressed while a request was in flight.
export const queuedKeys = [];

export function replayKeys() {
  while (queuedKeys.length && !ui.busy) onKeyDown(queuedKeys.shift());
}
