// Client-side state: the latest server state and the per-file view state
// (mode, cursors) that the TUI keeps in its App, plus derived views.

export const token = new URLSearchParams(location.hash.slice(1)).get("token") || "";

export let st = null; // latest server state (see stateJSON in view.go)

// setState replaces the server state (ES module bindings are read-only for
// importers).
export function setState(state) {
  st = state;
}

// hooks are calls into higher modules, set by main.js, so that lower modules
// never import them (the modules form no import cycle).
export const hooks = { render() {} };

// fileUIDefaults returns the per-file view state, reset for every file like
// a fresh TUI App.
export const fileUIDefaults = () => ({
  mode: "normal", // normal | comment | commentList | confirm | help | search | lineSelect
  sidebarOpen: false, // narrow windows: the section list shown over the content
  cursor: null, // selected section id
  fullView: false,
  rawView: false,
  lineCursor: 0, // index into file.lines
  anchor: -1, // visual selection anchor (index into file.lines)
  query: "",
  matches: null, // Set of the section IDs the search shows (null: no filter)
  collapsed: new Set(),
  leftRatio: 20, // list : content = 2 : 8 (wider screens than the TUI)
  pendingG: false,
  editor: null, // { id, sectionId, startLine, endLine, side, label, deco, body, fromList }
  list: null, // { sectionId, cursor }
  confirm: null, // "submit" | "quit"
});

export const ui = {
  finished: false,
  theme: "dark",
  // file picker (phase "pick")
  picker: { cursor: 0, selected: new Set() },
  seq: -1, // the file the per-file state belongs to
  ...fileUIDefaults(),
  busy: false, // a request is in flight
  pendingScroll: null, // scroll to apply after the next render
  spyPaused: false, // the full view does not follow scrolling until the user scrolls
};

export const file = () => st.file;
export const section = (id) => file().sections.find((s) => s.id === id);
export const commentsOf = (id) => file().comments.filter((c) => c.sectionId === id);
export const isViewed = (id) => file().viewed.includes(id);
// The overview cannot be marked viewed and is not counted, as in the TUI.
export const isRealSection = (id) => id != null && id !== st.overviewId;
export const clamp = (n, lo, hi) => Math.min(Math.max(n, lo), hi);

export function lineRef(c) {
  if (!c.startLine) return "";
  return c.endLine && c.endLine !== c.startLine ? `L${c.startLine}-L${c.endLine}` : `L${c.startLine}`;
}

export const formatLabel = (label, deco) => (deco ? `${label} (${deco})` : label);

// clearSearch drops the search filter.
export function clearSearch() {
  ui.query = "";
  ui.matches = null;
}

// ancestorsOf returns the indices of the ancestors of file.sections[i],
// nearest first.
export function ancestorsOf(i) {
  const secs = file().sections;
  const out = [];
  let depth = secs[i].depth;
  for (let j = i - 1; j >= 0 && depth > 0; j--) {
    if (secs[j].depth < depth) {
      out.push(j);
      depth = secs[j].depth;
    }
  }
  return out;
}

// listSections returns the sections shown in the section list: all minus
// collapsed subtrees, or those the search shows (matched by the server like
// the TUI's filter: the matches with their ancestors and descendants).
export function listSections() {
  const out = [];
  let hiddenBelow = Infinity; // depth of a collapsed ancestor
  for (const s of file().sections) {
    if (s.depth > hiddenBelow) continue;
    hiddenBelow = Infinity;
    if (ui.matches && !ui.matches.has(s.id)) continue;
    out.push(s);
    if (ui.collapsed.has(s.id)) hiddenBelow = s.depth;
  }
  return out;
}

export function hasChildren(s) {
  const secs = file().sections;
  const i = secs.indexOf(s);
  return i + 1 < secs.length && secs[i + 1].depth > s.depth;
}

// visibleLines returns the indices of file.lines the raw view shows: every
// line in the full view; in the section view, the selected section's lines.
export function visibleLines() {
  const f = file();
  const all = f.lines.map((_, i) => i);
  if (ui.fullView) return all;
  const s = section(ui.cursor);
  if (!s) return [];
  return all.filter((i) => f.lines[i].section === s.id);
}

// selectionRange returns the lines a line comment would cover: the cursor
// line, or the visual selection restricted to the cursor's diff side
// (a comment covers one side only).
export function selectionRange() {
  const f = file();
  const cur = f.lines[ui.lineCursor];
  if (!cur) return null;
  const side = cur.side || "";
  if (ui.anchor < 0) return { first: ui.lineCursor, last: ui.lineCursor, side };
  const lo = Math.min(ui.anchor, ui.lineCursor);
  const hi = Math.max(ui.anchor, ui.lineCursor);
  let first = -1;
  let last = -1;
  for (let i = lo; i <= hi; i++) {
    if ((f.lines[i].side || "") !== side) continue;
    if (first < 0) first = i;
    last = i;
  }
  return first < 0 ? null : { first, last, side };
}

export function inSelection(range, i) {
  return range && i >= range.first && i <= range.last && (file().lines[i].side || "") === range.side;
}

export function clampCursorToList() {
  const list = listSections();
  if (list.length && !list.some((s) => s.id === ui.cursor)) ui.cursor = list[0].id;
}
