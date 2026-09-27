// Server calls. Every successful change returns the whole session state,
// which is applied and rendered.

import { $, h, toast } from "./dom.js";
import { token, st, setState, ui, hooks, resetFileUI } from "./state.js";

// request sends an API request. It returns null when the command is gone,
// after switching the page to the end screen.
async function request(method, path, body) {
  try {
    return await fetch(path, {
      method,
      headers: { "X-Commd-Token": token, "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    showEnd("Review session ended", "commd is no longer running (it was stopped in the terminal). Nothing more can be saved.");
    return null;
  }
}

// errorOf returns the error message of a failed response.
async function errorOf(res) {
  return (await res.json().catch(() => ({}))).error || `${res.status} ${res.statusText}`;
}

// api calls the server and applies the returned state. A conflict means the
// session moved on (e.g. from another tab): the state is reloaded.
export async function api(method, path, body) {
  const res = await request(method, path, body);
  if (!res) return false;
  if (!res.ok) {
    toast(await errorOf(res));
    if (res.status === 409) await load();
    return false;
  }
  apply(await res.json());
  return true;
}

// fileAPI calls an endpoint of the file under review.
export const fileAPI = (method, path, body) => api(method, `/api/files/${st.seq}${path}`, body);

export async function load() {
  const res = await request("GET", "/api/state");
  if (!res) return;
  if (!res.ok) {
    showEnd("Cannot load the review", await errorOf(res));
    return;
  }
  apply(await res.json());
}

// searchSections returns the IDs of the sections a search for query shows,
// or null when the search failed (e.g. the file is no longer under review).
// It changes nothing on the server, so it runs outside the request queue and
// typing is never held up.
export async function searchSections(query) {
  const res = await request("GET", `/api/files/${st.seq}/search?q=${encodeURIComponent(query)}`);
  if (!res || !res.ok) return null;
  return (await res.json()).sections;
}

// apply takes a new server state. A new file starts with fresh view state,
// like the TUI creating a new App per file.
function apply(state) {
  setState(state);
  if (st.phase === "done") {
    showEnd("Review finished", "commd has the result. You can close this tab.");
    return;
  }
  if (st.phase === "review" && st.seq !== ui.seq) resetFileUI();
  hooks.render();
}

function showEnd(title, detail) {
  if (ui.finished) return;
  ui.finished = true;
  $("#modal-root").replaceChildren();
  $("#app").replaceChildren(h("div", { class: "done" }, h("h1", {}, title), h("p", {}, detail)));
  document.title = `commd — ${title}`;
}

// Input (keys and clicks) that arrives while a request is in flight is
// queued and replayed in order afterwards: the TUI never drops keys.
const queuedInput = [];
let replaying = false;

// inputDeferred reports whether new input must wait: during a request, and
// until the input queued meanwhile has been replayed, so the order is kept.
export const inputDeferred = () => ui.busy || (queuedInput.length > 0 && !replaying);

// deferInput queues run to replay after the request.
export function deferInput(run) {
  queuedInput.push(run);
}

// send runs one request at a time, then replays the input queued meanwhile.
export async function send(run) {
  if (ui.busy) return false;
  ui.busy = true;
  try {
    return await run();
  } finally {
    ui.busy = false;
    // After the caller has applied the response.
    setTimeout(replayInput, 0);
  }
}

function replayInput() {
  replaying = true;
  try {
    while (queuedInput.length && !ui.busy) queuedInput.shift()();
  } finally {
    replaying = false;
  }
}
