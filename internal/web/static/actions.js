// Actions: what keys and clicks do, mirroring the TUI App methods.

import { $, toast } from "./dom.js";
import { st, ui, hooks, file, section, commentsOf, isViewed, isRealSection, clamp, ancestorsOf, clearSearch, listSections, visibleLines, edgeLine, selectionRange, clampCursorToList } from "./state.js";
import { api, fileAPI, send, inputDeferred, deferInput, searchSections, reload } from "./api.js";

// guarded runs a mouse action only in the modes where the TUI would
// accept the equivalent key, then re-renders. Like keys, a click during a
// request runs once the request is done.
export function guarded(fn, fromAnyPane = false) {
  if (inputDeferred()) {
    deferInput(() => guarded(fn, fromAnyPane));
    return;
  }
  if (ui.mode !== "normal" && !(fromAnyPane && ui.mode === "search")) {
    refuseAction();
    return;
  }
  if (ui.mode === "search") closeSearch(true);
  fn();
  hooks.render();
}

// refuseAction tells why a click does nothing in the current mode.
export function refuseAction() {
  toast(ui.mode === "comment" ? "Save (Ctrl+S) or cancel (Esc) the comment first." : "Finish the current action first (Esc).");
}

export function toggleTheme() {
  ui.theme = ui.theme === "dark" ? "light" : "dark";
  document.documentElement.dataset.theme = ui.theme;
}

// moveCursorBy moves the section cursor within the listed sections.
export function moveCursorBy(delta) {
  const list = listSections();
  if (!list.length) return;
  const i = list.findIndex((s) => s.id === ui.cursor);
  const next = clamp((i < 0 ? 0 : i) + delta, 0, list.length - 1);
  moveCursorTo(list[next].id);
}

// moveCursorTo selects a section and brings the right pane to it
// (refreshAfterCursorMove in the TUI).
export function moveCursorTo(id) {
  ui.cursor = id;
  if (!ui.fullView) {
    showSectionEdge(false);
  } else if (ui.rawView) {
    const idx = file().lines.findIndex((l) => l.section === id);
    if (idx >= 0) ui.lineCursor = idx;
  } else {
    ui.pendingScroll = () => document.querySelector(`#content [data-section="${CSS.escape(id)}"]`)?.scrollIntoView({ block: "start" });
    ui.spyPaused = true;
  }
}

// showSectionEdge shows the start (or with atEnd the end) of what the right
// pane shows: the line cursor goes to the first or last visible line in the
// raw view, the rendered view scrolls to its top or bottom.
function showSectionEdge(atEnd) {
  if (ui.rawView) {
    ui.lineCursor = edgeLine(atEnd);
    return;
  }
  ui.pendingScroll = () => {
    const el = $("#content");
    el.scrollTop = atEnd ? el.scrollHeight : 0;
  };
}

export function toggleExpand() {
  if (!ui.cursor) return;
  if (ui.collapsed.has(ui.cursor)) ui.collapsed.delete(ui.cursor);
  else ui.collapsed.add(ui.cursor);
}

export function toggleFull() {
  ui.fullView = !ui.fullView;
  if (ui.rawView) syncLineCursorIntoView();
  else moveCursorTo(ui.cursor);
}

export function toggleRaw() {
  ui.rawView = !ui.rawView;
  ui.anchor = -1;
  if (ui.rawView) showSectionEdge(false);
}

// syncLineCursorIntoView keeps the line cursor on a visible line.
function syncLineCursorIntoView() {
  const vis = visibleLines();
  if (vis.length && !vis.includes(ui.lineCursor)) ui.lineCursor = vis[0];
}

// syncSectionFromLineCursor selects the section of the line under the
// cursor, as the TUI does when the line cursor moves.
export function syncSectionFromLineCursor() {
  const l = file().lines[ui.lineCursor];
  if (l && section(l.section)) ui.cursor = l.section;
}

export function moveLineCursor(delta) {
  const vis = visibleLines();
  if (!vis.length) return;
  const i = Math.max(vis.indexOf(ui.lineCursor), 0);
  ui.lineCursor = vis[clamp(i + delta, 0, vis.length - 1)];
  syncSectionFromLineCursor();
}

// verticalMove moves the view down (positive) or up: the line cursor by rows
// in the raw view, the scroll position by px otherwise. There are no panes
// to focus in the browser: in the section view, moving on at the end (or
// the start) of a section steps into the next (or previous) one.
export function verticalMove(rows, px) {
  if (ui.rawView) {
    const vis = visibleLines();
    const edge = rows > 0 ? vis.at(-1) : vis[0];
    if (!ui.fullView && (!vis.length || ui.lineCursor === edge)) stepSection(Math.sign(rows));
    else moveLineCursor(rows);
    return;
  }
  const el = $("#content");
  if (!ui.fullView && atScrollEdge(el, px)) stepSection(Math.sign(px));
  else scrollDetail(el, px);
}

// jumpToEdge goes to the top (dir < 0) or the end of the whole document: in
// the section view, to its first or last section, even when collapsed or
// filtered out of the list (see revealSection).
export function jumpToEdge(dir) {
  const secs = file().sections;
  if (!ui.fullView && secs.length) {
    const target = dir < 0 ? secs[0] : secs.at(-1);
    revealSection(target.id);
    moveCursorTo(target.id);
  }
  if (ui.rawView) {
    showSectionEdge(dir > 0);
    syncSectionFromLineCursor();
  } else if (ui.fullView) {
    const el = $("#content");
    scrollDetail(el, dir < 0 ? -el.scrollHeight : el.scrollHeight);
  } else {
    showSectionEdge(dir > 0);
  }
}

// revealSection makes a section show in the list: its collapsed ancestors are
// expanded, and a search filter it does not match is cleared.
function revealSection(id) {
  const secs = file().sections;
  const i = secs.findIndex((s) => s.id === id);
  if (i < 0) return;
  for (const j of ancestorsOf(i)) ui.collapsed.delete(secs[j].id);
  if (!listSections().some((s) => s.id === id)) clearSearch();
}

// atScrollEdge reports whether el cannot scroll any further in the
// direction of dy.
export function atScrollEdge(el, dy) {
  // 1px of slack: half-page scrolls leave fractional positions.
  return dy > 0 ? el.scrollTop + el.clientHeight >= el.scrollHeight - 1 : el.scrollTop <= 1;
}

// stepSection selects the next (dir > 0) or previous listed section, showing
// its start when moving down and its end when moving up.
export function stepSection(dir) {
  const prev = ui.cursor;
  moveCursorBy(dir);
  if (ui.cursor !== prev) showSectionEdge(dir < 0);
}

export const lineHeight = 20; // .lines line-height in style.css
export const pageRows = () => Math.max(Math.floor(($("#content")?.clientHeight || 400) / lineHeight), 1);

function scrollDetail(el, dy) {
  el.scrollBy({ top: dy });
  ui.spyPaused = false;
  // The re-render after the key replaces the pane before its scroll event
  // arrives, so follow the scroll here.
  syncCursorToScroll();
}

// syncCursorToScroll selects the section at the top of the full rendered
// view (syncCursorToScroll in the TUI). Reports whether the cursor moved.
export function syncCursorToScroll() {
  const content = $("#content");
  if (!content || !ui.fullView || ui.rawView) return false;
  const top = content.getBoundingClientRect().top + 24;
  const parts = content.querySelectorAll("[data-section]");
  // Until a section reaches the top (e.g. scrolled to the very top, below
  // the page padding), the first one is current.
  let current = parts[0]?.dataset.section ?? null;
  for (const block of parts) {
    if (block.getBoundingClientRect().top <= top) current = block.dataset.section;
    else break;
  }
  if (!current || current === ui.cursor) return false;
  ui.cursor = current;
  return true;
}

// scrollHorizontal scrolls the wide blocks of the rendered view (code and
// tables), the browser counterpart of the TUI detail pane's x offset.
export function scrollHorizontal(fn) {
  if (ui.rawView) return; // the TUI scrolls the rendered viewport only
  for (const el of document.querySelectorAll("#content pre, #content .markdown table")) fn(el);
}

// The section list takes 10-50% of the width, as in the TUI.
const minLeftRatio = 10;
const maxLeftRatio = 50;

export function resizeLeft(delta) {
  const next = ui.leftRatio + delta;
  if (window.innerWidth < 720 || next < minLeftRatio || next > maxLeftRatio) return;
  ui.leftRatio = next;
}

// startResize lets the pane border be dragged (mouse only). The drag ends on
// mouseup, and also when the window loses focus or a move arrives with the
// button already released (a mouseup outside the window), so its listeners
// never outlive it.
export function startResize(ev) {
  ev.preventDefault();
  const resizer = ev.currentTarget;
  resizer.classList.add("dragging");
  const drag = new AbortController();
  const end = () => {
    resizer.classList.remove("dragging");
    drag.abort();
  };
  const move = (e) => {
    if (e.buttons === 0) {
      end();
      return;
    }
    ui.leftRatio = clamp((e.clientX / window.innerWidth) * 100, minLeftRatio, maxLeftRatio);
    $("#sidebar").style.width = `${ui.leftRatio}%`;
  };
  document.addEventListener("mousemove", move, { signal: drag.signal });
  document.addEventListener("mouseup", end, { signal: drag.signal });
  window.addEventListener("blur", end, { signal: drag.signal });
}

// reloadFile reads the file again (R), as the TUI does: the view stays on
// the selected section, keeping its full/raw view and pane width.
export function reloadFile() {
  const { cursor, fullView, rawView, leftRatio } = ui;
  send(async () => {
    const r = await reload();
    if (!r) return;
    if (r.changed) {
      // The page reset its view state for the new content.
      Object.assign(ui, { fullView, rawView: rawView && file().lines.length > 0, leftRatio });
      moveCursorTo(r.sections?.[cursor] ?? ui.cursor);
      hooks.render();
    }
    toast(r.message);
  });
}

export function toggleViewed(id) {
  if (!isRealSection(id) || !section(id)) return;
  send(() => fileAPI("PUT", `/viewed/${encodeURIComponent(id)}`, { viewed: !isViewed(id) }));
}

// openSectionEditor opens the editor for a section comment; it is offered in
// the rendered view only, as in the TUI.
export function openSectionEditor(id) {
  if (!id) return;
  openEditor({ sectionId: id });
}

// startLineSelect starts a visual line selection from anchor to cursor
// (indices into file.lines).
export function startLineSelect(anchor, cursor) {
  ui.anchor = anchor;
  ui.lineCursor = cursor;
  ui.mode = "lineSelect";
}

export function exitLineSelect() {
  ui.anchor = -1;
  ui.mode = "normal";
}

export function openLineEditor() {
  const f = file();
  const r = selectionRange();
  if (!r || !visibleLines().length) return;
  const start = f.lines[r.first].line;
  const end = f.lines[r.last].line;
  ui.anchor = -1;
  openEditor({ sectionId: f.lines[r.first].section, startLine: start, endLine: end, side: r.side });
}

// openEditor opens the editor for an existing comment (c has an id, and it
// was opened from the comment list) or a new one on c's target.
function openEditor(c) {
  ui.editor = {
    id: c.id || null,
    sectionId: c.sectionId,
    startLine: c.startLine || 0,
    endLine: c.endLine || 0,
    side: c.side || "",
    label: Math.max(st.labels.indexOf(c.action || st.defaultLabel), 0),
    deco: Math.max(st.decorations.indexOf(c.decoration || ""), 0),
    body: c.body || "",
  };
  ui.mode = "comment";
}

export async function saveEditor() {
  const e = ui.editor;
  const payload = {
    sectionId: e.sectionId,
    action: st.labels[e.label],
    decoration: st.decorations[e.deco],
    body: e.body,
    startLine: e.startLine,
    endLine: e.endLine,
    side: e.side,
  };
  // The server applies the rule the TUI shares: an empty new comment is
  // dropped, and emptying an existing one deletes it.
  const ok = await send(() =>
    e.id ? fileAPI("PATCH", `/comments/${encodeURIComponent(e.id)}`, payload) : fileAPI("POST", "/comments", payload),
  );
  if (ok) closeEditor();
  hooks.render();
}

export function closeEditor() {
  const e = ui.editor;
  ui.editor = null;
  if (e && e.id) reopenList(e.sectionId);
  else ui.mode = "normal";
}

export function cycle(n, delta, len) {
  return (n + delta + len) % len;
}

export function openList(sectionId) {
  if (!sectionId || !commentsOf(sectionId).length) return;
  ui.list = { sectionId, cursor: 0 };
  ui.mode = "commentList";
}

export function closeList() {
  ui.list = null;
  ui.mode = "normal";
}

// reopenList shows the list after its comments changed, or returns to
// normal mode when none remain.
function reopenList(sectionId) {
  const n = commentsOf(sectionId).length;
  if (!n) {
    closeList();
    return;
  }
  const cursor = ui.list && ui.list.sectionId === sectionId ? Math.min(ui.list.cursor, n - 1) : 0;
  ui.list = { sectionId, cursor };
  ui.mode = "commentList";
}

export function editFromList() {
  const c = commentsOf(ui.list.sectionId)[ui.list.cursor];
  if (!c) return;
  openEditor(c);
  hooks.render();
}

export async function deleteFromList() {
  const c = commentsOf(ui.list.sectionId)[ui.list.cursor];
  if (!c) return;
  const sectionId = ui.list.sectionId;
  if (await send(() => fileAPI("DELETE", `/comments/${encodeURIComponent(c.id)}`))) reopenList(sectionId);
  hooks.render();
}

export function openHelp() {
  ui.mode = "help";
}

export function closeHelp() {
  ui.mode = "normal";
}

export function openConfirm(kind) {
  ui.confirm = kind;
  ui.mode = "confirm";
}

export function closeModal() {
  ui.mode = "normal";
  ui.confirm = null;
  hooks.render();
}

export async function executeConfirm() {
  const submit = ui.confirm === "submit";
  ui.mode = "normal";
  ui.confirm = null;
  await finish(submit);
}

// finish ends the current file: submitted (or approved) or quit/skipped.
export async function finish(submit) {
  await send(() => fileAPI("POST", "/finish", { action: submit ? "submit" : "quit" }));
  hooks.render();
}

export function openSearch() {
  ui.mode = "search";
  clearSearch();
  ui.sidebarOpen = true; // narrow windows: the list holds the search input
  clampCursorToList();
}

// runSearch filters the section list by the query typed so far. The server
// matches the sections, as the TUI's filter does; a response that arrives
// after the query changed again is dropped. The search input is not
// re-rendered, so typing (including IME composition) is not interrupted.
export async function runSearch() {
  const query = ui.query;
  const matches = query ? await searchSections(query) : null;
  if (ui.query !== query || (query && !matches)) return;
  ui.matches = matches && new Set(matches);
  clampCursorToList();
  moveCursorTo(ui.cursor);
  hooks.refreshPanes();
}

// closeSearch leaves search mode: keep=true keeps the filter (Enter),
// otherwise it is cleared (Esc).
export function closeSearch(keep) {
  ui.mode = "normal";
  ui.sidebarOpen = false;
  if (!keep) clearSearch();
  clampCursorToList();
  moveCursorTo(ui.cursor);
}

export function togglePick(i) {
  const sel = ui.picker.selected;
  if (sel.has(i)) sel.delete(i);
  else sel.add(i);
  hooks.render();
}

// toggleAllPicks selects every file, or none when all are selected.
export function toggleAllPicks() {
  const p = ui.picker;
  p.selected = p.selected.size === st.pick.length ? new Set() : new Set(st.pick.map((_, i) => i));
}

export function confirmPick() {
  send(() => api("POST", "/api/pick", { paths: st.pick.filter((_, i) => ui.picker.selected.has(i)) }));
}

export function cancelPick() {
  // Choosing nothing ends the session, as cancelling the TUI picker does.
  send(() => api("POST", "/api/pick", { paths: [] }));
}

