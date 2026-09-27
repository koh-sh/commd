// Key bindings per mode, mirroring the TUI key handlers.

import { $ } from "./dom.js";
import { st, ui, file, commentsOf, clamp, visibleLines } from "./state.js";
import { inputDeferred, deferInput } from "./api.js";
import { render, updateEditorChrome } from "./render.js";
import { moveCursorBy, jumpToEdge, toggleExpand, toggleFull, toggleRaw, moveLineCursor, verticalMove, lineHeight, pageRows, scrollHorizontal, resizeLeft, toggleViewed, openSectionEditor, startLineSelect, exitLineSelect, openLineEditor, saveEditor, closeEditor, cycle, openList, closeList, editFromList, deleteFromList, openHelp, closeHelp, openConfirm, closeModal, executeConfirm, finish, openSearch, closeSearch, togglePick, toggleAllPicks, confirmPick, cancelPick } from "./actions.js";

function onPickerKey(ev) {
  const p = ui.picker;
  const n = st.pick.length;
  const k = ev.key;
  const ctrlC = ev.ctrlKey && k === "c" && !hasTextSelection(); // with a selection it copies
  if (k === "q" || keyName(ev) === "esc" || ctrlC) cancelPick();
  else if (k === "Enter") confirmPick();
  else if (k === "j" || k === "ArrowDown") p.cursor = clamp(p.cursor + 1, 0, n - 1);
  else if (k === "k" || k === "ArrowUp") p.cursor = clamp(p.cursor - 1, 0, n - 1);
  else if (k === " ") togglePick(p.cursor);
  else if (k === "a") toggleAllPicks();
  else return;
  ev.preventDefault();
  render();
}

// keyName normalizes a key event to the TUI's key names.
function keyName(ev) {
  // ctrl+[ (and ctrl+{) sends Esc in a terminal, so it is Esc here too.
  if (ev.ctrlKey && (ev.key === "[" || ev.key === "{")) return "esc";
  const map = { ArrowUp: "up", ArrowDown: "down", ArrowLeft: "left", ArrowRight: "right", Enter: "enter", Escape: "esc", Tab: "tab", " ": "space" };
  let k = map[ev.key] || ev.key;
  if (ev.ctrlKey && k.length === 1) k = `ctrl+${k.toLowerCase()}`;
  if (ev.shiftKey && k === "tab") k = "shift+tab";
  return k;
}

function hasTextSelection() {
  const sel = window.getSelection();
  return Boolean(sel && !sel.isCollapsed && sel.toString());
}

export function onKeyDown(ev) {
  if (ui.finished || !st) return;
  // Keys that belong to an IME composition (e.g. Enter confirming Japanese
  // input) are not shortcuts.
  if (ev.isComposing || ev.keyCode === 229) return;
  // Keys pressed during a request are replayed after it, in order.
  if (inputDeferred()) {
    ev.preventDefault();
    const { key, ctrlKey, shiftKey, metaKey, altKey } = ev;
    const replayed = { key, ctrlKey, shiftKey, metaKey, altKey, replayed: true, target: document.body, preventDefault() {} };
    deferInput(() => onKeyDown(replayed));
    return;
  }
  if (st.phase === "pick") {
    onPickerKey(ev);
    return;
  }
  if (ev.metaKey && !(ev.key === "Enter" && ui.mode === "comment")) return; // browser shortcuts
  if (ev.altKey) return;
  const k = keyName(ev);
  // ctrl+c quits from every mode (skips the file in a multi-file review),
  // like the TUI; with text selected it stays the copy shortcut.
  if (k === "ctrl+c") {
    const ta = ev.target instanceof HTMLTextAreaElement && ev.target.selectionStart !== ev.target.selectionEnd;
    if (hasTextSelection() || ta) return;
    ev.preventDefault();
    finish(false);
    return;
  }
  const handler = { normal: onNormalKey, comment: onCommentKey, commentList: onListKey, confirm: onConfirmKey, help: onHelpKey, search: onSearchKey, lineSelect: onLineSelectKey }[ui.mode];
  // Handlers return false for keys left to the browser, "keep" for keys
  // they handled without a state change that needs a re-render.
  const result = handler(k, ev);
  if (result === false) {
    // The browser types keys we leave to it, but not replayed ones.
    if (ev.replayed) typeReplayed(ev);
    return;
  }
  ev.preventDefault();
  if (result !== "keep") render();
}

// Handlers return false for keys they leave to the browser.

function onCommentKey(k, ev) {
  const e = ui.editor;
  if (k === "ctrl+s" || (k === "enter" && (ev.ctrlKey || ev.metaKey))) {
    saveEditor();
    return true;
  }
  if (k === "esc") {
    closeEditor();
    return true;
  }
  if (k === "ctrl+d") e.deco = cycle(e.deco, 1, st.decorations.length);
  else if (k === "shift+tab") e.label = cycle(e.label, -1, st.labels.length);
  else if (k === "tab") e.label = cycle(e.label, 1, st.labels.length);
  if (k === "ctrl+d" || k === "tab" || k === "shift+tab") {
    updateEditorChrome();
    return "keep";
  }
  // Typing goes to the editor even if focus wandered off it.
  const ta = $("#editor-body");
  if (ta && ev.target !== ta) ta.focus();
  return false;
}

function onListKey(k) {
  const list = ui.list;
  const n = commentsOf(list.sectionId).length;
  switch (k) {
    case "esc":
      closeList();
      break;
    case "k":
    case "up":
      list.cursor = clamp(list.cursor - 1, 0, n - 1);
      break;
    case "j":
    case "down":
      list.cursor = clamp(list.cursor + 1, 0, n - 1);
      break;
    case "e":
      editFromList();
      break;
    case "d":
      deleteFromList();
      break;
  }
  return true; // other keys are ignored in the list
}

function onConfirmKey(k) {
  if (k === "y" || k === "Y") executeConfirm();
  else if (["n", "N", "q", "esc"].includes(k)) closeModal();
  return true;
}

function onHelpKey(k) {
  if (["esc", "?", "enter", "q"].includes(k)) closeHelp();
  return true;
}

function onSearchKey(k) {
  switch (k) {
    case "enter":
      closeSearch(true);
      return true;
    case "esc":
      closeSearch(false);
      return true;
    case "up":
      moveCursorBy(-1);
      return true;
    case "down":
      moveCursorBy(1);
      return true;
    case "tab":
    case "shift+tab":
      return true; // keep the focus in the search input, as the TUI does
  }
  return false; // typing goes to the search input
}

function onLineSelectKey(k) {
  switch (k) {
    case "k":
    case "up":
      moveLineCursor(-1);
      break;
    case "j":
    case "down":
      moveLineCursor(1);
      break;
    case "c":
      openLineEditor();
      break;
    case "esc":
      exitLineSelect();
      break;
  }
  return true;
}

function onNormalKey(k) {
  const el = $("#content");
  // gg chord
  if (ui.pendingG) {
    ui.pendingG = false;
    if (k === "g") {
      jumpToEdge(-1); // top of the document
      return true;
    }
  }
  const half = Math.floor(pageRows() / 2);
  switch (k) {
    case "g":
      ui.pendingG = true;
      return true;
    case "G":
      jumpToEdge(1); // end of the document
      return true;
    case "q":
      openConfirm("quit");
      return true;
    case "?":
      openHelp();
      return true;
    case "tab":
      return true; // no panes to switch in the browser
    case "s":
      openConfirm("submit");
      return true;
    case ">":
      resizeLeft(5);
      return true;
    case "<":
      resizeLeft(-5);
      return true;
    case "f":
      toggleFull();
      return true;
    case "r":
      toggleRaw();
      return true;
    case "l":
    case "right":
      scrollHorizontal((e) => e.scrollBy({ left: 32 }));
      return "keep"; // a re-render would reset the scroll offsets
    case "h":
    case "left":
      scrollHorizontal((e) => e.scrollBy({ left: -32 }));
      return "keep";
    case "H":
      scrollHorizontal((e) => (e.scrollLeft = 0));
      return "keep";
    case "L":
      scrollHorizontal((e) => (e.scrollLeft = e.scrollWidth));
      return "keep";
    // Vertical keys move the content (or the raw line cursor) and step into
    // the next or previous section at the edges.
    case "j":
    case "down":
      verticalMove(1, lineHeight);
      return true;
    case "k":
    case "up":
      verticalMove(-1, -lineHeight);
      return true;
    case "ctrl+d":
      verticalMove(half, el.clientHeight / 2);
      return true;
    case "ctrl+u":
      verticalMove(-half, -el.clientHeight / 2);
      return true;
    case "ctrl+f":
      verticalMove(pageRows(), el.clientHeight);
      return true;
    case "ctrl+b":
      verticalMove(-pageRows(), -el.clientHeight);
      return true;
    case "enter":
      toggleExpand();
      return true;
    case "/":
      openSearch();
      return true;
    case "v":
      toggleViewed(ui.cursor);
      return true;
  }
  return ui.rawView ? onLineKey(k) : sectionAction(k);
}

// sectionAction handles c and C on the selected section in the rendered view.
function sectionAction(k) {
  switch (k) {
    case "c":
      openSectionEditor(ui.cursor);
      return true;
    case "C":
      openList(ui.cursor);
      return true;
  }
  return false;
}

// onLineKey handles the raw view's line comment keys.
function onLineKey(k) {
  switch (k) {
    case "c":
      openLineEditor();
      return true;
    case "V":
      if (visibleLines().length) startLineSelect(ui.lineCursor, ui.lineCursor);
      return true;
    case "C": {
      const l = file().lines[ui.lineCursor];
      if (l) openList(l.section);
      return true;
    }
  }
  return false;
}

// typeReplayed types a replayed key into the focused text field, as the
// browser would have if the key had not been queued during a request.
function typeReplayed(ev) {
  const el = document.activeElement;
  if (!(el instanceof HTMLTextAreaElement || el instanceof HTMLInputElement) || el.readOnly) return;
  if (ev.ctrlKey || ev.metaKey || ev.altKey) return;
  const start = el.selectionStart ?? el.value.length;
  const end = el.selectionEnd ?? start;
  if (ev.key.length === 1) {
    el.setRangeText(ev.key, start, end, "end");
  } else if (ev.key === "Enter" && el instanceof HTMLTextAreaElement) {
    el.setRangeText("\n", start, end, "end");
  } else if (ev.key === "Backspace") {
    el.setRangeText("", start === end ? Math.max(start - 1, 0) : start, end, "end");
  } else {
    return;
  }
  el.dispatchEvent(new Event("input", { bubbles: true }));
}
