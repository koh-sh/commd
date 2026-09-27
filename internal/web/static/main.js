// commd browser review UI: entry point. The page mirrors the TUI
// (internal/tui): the same modes, key bindings, dialogs and status bar, plus
// mouse support. Unlike the TUI there is no pane focus: vertical keys move
// the content and step between sections at its edges. All review state lives on the server
// (internal/web); every change goes through the API and the page re-renders
// from the returned state.
//
// Modules, lowest first (each imports only lower ones): state (client state
// and derived views of the server state), dom (small DOM helpers), text
// (dialog and status bar text), api (server calls and the input queue),
// actions (what keys and clicks do), picker (the file picker), render (DOM),
// keys (key bindings per mode), mouse. Lower modules reach render through
// state.hooks.

import { st, ui, hooks } from "./state.js";
import { toast } from "./dom.js";
import { reload } from "./api.js";
import { render, refreshPanes } from "./render.js";
import { onKeyDown } from "./keys.js";
import { onLineMouseDown, onLineMouseOver, onMouseUp, onContentScroll, onClick, onWheel } from "./mouse.js";

function bind() {
  document.addEventListener("keydown", onKeyDown);
  document.addEventListener("mousedown", onLineMouseDown);
  document.addEventListener("mouseover", onLineMouseOver);
  document.addEventListener("mouseup", onMouseUp);
  document.addEventListener("click", onClick);
  document.addEventListener("scroll", onContentScroll, true);
  document.addEventListener("wheel", onWheel, { passive: true });
  document.addEventListener("touchmove", () => (ui.spyPaused = false), { passive: true });
}

hooks.render = render;
hooks.refreshPanes = refreshPanes;
bind();
reload().then((r) => {
  if (r && (r.changed || r.failed)) toast(r.message); // opening the page is quiet unless the file changed
  if (st) {
    ui.theme = st.theme === "light" ? "light" : "dark";
    if (st.phase === "pick") ui.picker.selected = new Set(st.pick.map((_, i) => i)); // all selected, like the TUI
    render();
  }
});
