// Mouse support (browser only): line selection by dragging line numbers,
// stepping sections by wheel at the edges, and section following in the full
// view. The pane border drag is startResize in actions.js.

import { $ } from "./dom.js";
import { st, ui } from "./state.js";
import { render, refreshCursor, visibleTop } from "./render.js";
import { refuseAction, syncSectionFromLineCursor, openLineEditor, syncCursorToScroll, atScrollEdge, stepSection, lineHeight } from "./actions.js";

let dragging = false;

export function onLineMouseDown(ev) {
  const cell = ev.target.closest("td.num");
  if (!cell || ev.button !== 0) return;
  ev.preventDefault();
  if (ui.mode !== "normal" && ui.mode !== "lineSelect") {
    refuseAction();
    return;
  }
  const idx = Number(cell.dataset.idx);
  ui.anchor = ev.shiftKey ? ui.lineCursor : idx;
  ui.lineCursor = idx;
  ui.mode = "lineSelect";
  dragging = true;
  render();
}

export function onLineMouseOver(ev) {
  if (!dragging) return;
  const cell = ev.target.closest("td.num");
  if (!cell) return;
  const idx = Number(cell.dataset.idx);
  // Re-rendering replaces the cell under the pointer, which fires another
  // mouseover; render only when the selection actually changes.
  if (idx === ui.lineCursor) return;
  ui.lineCursor = idx;
  render();
}

export function onMouseUp() {
  if (!dragging) return;
  dragging = false;
  syncSectionFromLineCursor();
  openLineEditor(); // mouse selection = V + j/k + c
  render();
}

// onContentScroll keeps the cursors with the scroll: in the full rendered
// view the section cursor follows the section at the top (syncCursorToScroll
// in the TUI) unless a section was just chosen; in the raw view the line
// cursor stays on screen.
let spyQueued = false;
export function onContentScroll() {
  if (!st || st.phase !== "review" || spyQueued) return;
  if (ui.rawView ? ui.mode !== "normal" : !ui.fullView || ui.spyPaused) return;
  spyQueued = true;
  requestAnimationFrame(() => {
    spyQueued = false;
    if (ui.rawView) keepLineCursorInView();
    else if (syncCursorToScroll()) render(); // render keeps the scroll position
  });
}

// keepLineCursorInView moves the raw view's line cursor back onto the screen
// when scrolling (wheel, scrollbar) leaves it behind, like vim and less: to
// the first visible line when it went off the top, the last when off the
// bottom. Keys then continue from where the reader is looking.
function keepLineCursorInView() {
  const content = $("#content");
  if (!content) return;
  const box = content.getBoundingClientRect();
  const top = visibleTop(content); // the same test scrollRowIntoView uses
  const visible = [...content.querySelectorAll("tr[data-idx]")].filter((row) => {
    const r = row.getBoundingClientRect();
    return r.top >= top - 1 && r.bottom <= box.bottom + 1;
  });
  const cur = content.querySelector("tr.cursor");
  if (!visible.length || (cur && visible.includes(cur))) return;
  const above = cur ? cur.getBoundingClientRect().top < top : true;
  ui.lineCursor = Number((above ? visible[0] : visible.at(-1)).dataset.idx);
  syncSectionFromLineCursor();
  refreshCursor();
}

export function onClick(ev) {
  // A clicked button must not keep the focus: Enter and Space would press it
  // again, while in the TUI those keys do not act on buttons.
  ev.target.closest("button, input[type=checkbox]")?.blur();
}

// onWheel lets scrolling run on across sections in the section view: a wheel
// event that finds the section already scrolled to its end (or start) moves
// to the next (or previous) section and keeps scrolling there, so momentum
// carries on as in one long page.
export function onWheel(ev) {
  const content = ev.target.closest?.("#content");
  if (!content || ev.deltaY === 0) return;
  if (!st || st.phase !== "review" || ui.mode !== "normal" || ui.fullView) return;
  if (!atScrollEdge(content, ev.deltaY)) return;
  const prev = ui.cursor;
  stepSection(Math.sign(ev.deltaY));
  if (ui.cursor === prev) return; // first or last section: nothing to move to
  render();
  // Carry this event's movement into the new section (deltaMode 1 = lines,
  // 2 = pages).
  const el = $("#content");
  const unit = ev.deltaMode === 1 ? lineHeight : ev.deltaMode === 2 ? el.clientHeight : 1;
  el.scrollBy({ top: ev.deltaY * unit });
}

