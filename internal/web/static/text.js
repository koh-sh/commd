// Text shown in the dialogs and the status bar, mirroring the TUI's.

import { plural } from "./dom.js";
import { ui, file, isViewed, isRealSection, lineRef, selectionRange } from "./state.js";

// confirmMessage is the question of the confirm dialog for ui.confirm, worded
// by the server as in the TUI.
export function confirmMessage() {
  return ui.confirm === "submit" ? file().confirmSubmit : file().confirmQuit;
}

// statusKeys returns the [key, label] entries of the status bar in the
// current mode.
export function statusKeys() {
  switch (ui.mode) {
    case "comment":
      return [["tab/S-tab", "label"], ["ctrl+d", "deco"], ["ctrl+s", "save"], ["esc", "cancel"]];
    case "commentList":
      return [["j/k", "navigate"], ["e", "edit"], ["d", "delete"], ["esc", "back"]];
    case "lineSelect":
      return [["j/k", "extend"], ["c", "comment"], ["esc", "cancel"]];
    case "search":
      return [["↑/↓", "navigate results"], ["enter", "confirm"], ["esc", "cancel"]];
  }
  const viewMode = ui.fullView ? "section" : "full";
  if (ui.rawView) {
    return [["r", "render"], ["f", viewMode], ["c", "comment"], ["V", "select"], ["C", "comments"], ["s", "submit"], ["?", "help"], ["q", "quit"]];
  }
  return [
    ["enter", "toggle"], ["f", viewMode], ["r", "raw"], ["c", "comment"], ["C", "comments"],
    ["v", "viewed"], ["/", "search"], ["s", "submit"], ["?", "help"], ["q", "quit"],
  ];
}

// statusIndicator returns the right side of the status bar: the selected
// range in visual mode, otherwise the position and progress.
export function statusIndicator() {
  if (ui.mode === "lineSelect") {
    const r = selectionRange();
    return r ? lineRef({ startLine: file().lines[r.first].line, endLine: file().lines[r.last].line }) : "";
  }
  if (["comment", "commentList", "search"].includes(ui.mode)) return "";
  const n = file().comments.length;
  const progress = n ? ` [${plural(n, "comment")}]` : "";
  // As in the TUI: the cursor position among all lines of the view.
  if (ui.rawView) return `L${ui.lineCursor + 1}/${file().lines.length}${progress}`;
  const real = file().sections.filter((s) => isRealSection(s.id));
  return `[${real.filter((s) => isViewed(s.id)).length}/${real.length} viewed]${progress}`;
}

export function helpText() {
  return `  Navigation:
  j/k, Up/Down   Scroll (raw view: move the line cursor); at the end
                  or start of a section, move to the next / previous one
  gg              Go to the top of the document
  G               Go to the end of the document
  Enter           Toggle expand/collapse
  f               Toggle full/section view
  r               Toggle raw source/rendered view
  h/l, Left/Right Scroll detail pane left/right
  H/L             Scroll detail to start/end
  Ctrl+D/Ctrl+U   Half page down/up
  Ctrl+F/Ctrl+B   Full page down/up
  >/<             Resize left pane

Review:
  c               Add comment on selected section
  C               Manage comments (edit/delete)
  v               Toggle viewed mark
  /               Search sections
  s               Submit review
  R               Reload the file (comments are kept)

Raw Source View (r to toggle):
  j/k             Move line cursor
  c               Add line comment at cursor
  V               Start visual line selection
  V + j/k + c     Comment on selected range
  Esc             Cancel visual selection
  C               Manage comments for section at cursor

Comment Editor:
  Tab             Cycle label (forward)
  Shift+Tab       Cycle label (reverse)
  Ctrl+D          Cycle decoration
  Ctrl+S          Save comment (also Ctrl/⌘+Enter)
  Esc             Cancel editing

Mouse (browser only):
  Click a section, line number, label or button; drag line
  numbers (or shift+click) to select a range; drag the pane
  border to resize. Scrolling runs on across sections: past the
  end (or start) of one it continues into the next (or previous).

Other:
  ?               Toggle this help
  q, Ctrl+C       Quit

Press Esc, Enter, ? or q to close this help.`;
}
