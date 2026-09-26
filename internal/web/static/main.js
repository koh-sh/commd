// commd browser review UI: entry point. The page mirrors the TUI
// (internal/tui): the same modes, key bindings, dialogs and status bar, plus
// mouse support. Unlike the TUI there is no pane focus: vertical keys move
// the content and step between sections at its edges. All review state lives on the server
// (internal/web); every change goes through the API and the page re-renders
// from the returned state.
//
// Modules: state (client state and derived views of the server state), api
// (server calls), render (DOM), actions (what keys and clicks do), keys
// (key bindings per mode), mouse, dom (small DOM helpers).

import { st, ui } from "./state.js";
import { load } from "./api.js";
import { render } from "./render.js";
import { onKeyDown } from "./keys.js";
import { onLineMouseDown, onLineMouseOver, onMouseUp, onContentScroll, onClick, onWheel } from "./mouse.js";

function bind() {
  document.addEventListener("keydown", onKeyDown);
  document.addEventListener("mousedown", onLineMouseDown);
  document.addEventListener("mouseover", onLineMouseOver);
  document.addEventListener("mouseup", onMouseUp);
  document.addEventListener("click", onClick);
  document.addEventListener("scroll", onContentScroll, true);
  for (const type of ["wheel", "touchmove"]) document.addEventListener(type, () => (ui.spyPaused = false), { passive: true });
  document.addEventListener("wheel", onWheel, { passive: true });
}

bind();
load().then(() => {
  if (st) {
    ui.theme = st.theme === "light" ? "light" : "dark";
    if (st.phase === "pick") ui.picker.selected = new Set(st.pick.map((_, i) => i)); // all selected, like the TUI
    render();
  }
});
