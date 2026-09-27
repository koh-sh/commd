// Rendering: builds the DOM from the server state and the view state.

import { $, h, button, plural } from "./dom.js";
import { st, ui, file, section, commentsOf, isViewed, isRealSection, lineRef, formatLabel, listSections, parentIds, visibleLines, selectionRange, inSelection } from "./state.js";
import { confirmMessage, statusKeys, statusIndicator, helpText } from "./text.js";
import { guarded, toggleTheme, moveCursorTo, toggleExpand, toggleFull, toggleRaw, toggleViewed, openSectionEditor, editFromList, deleteFromList, openHelp, openConfirm, closeModal, executeConfirm, openSearch, runSearch, startResize } from "./actions.js";
import { renderPicker } from "./picker.js";

export function render() {
  if (ui.finished || !st) return;
  document.documentElement.dataset.theme = ui.theme;
  if (st.phase === "pick") {
    renderPicker();
    return;
  }
  const f = file();
  const title = [f.title, `(${f.path})`].filter(Boolean).join(" ");
  document.title = `commd — ${f.path}`;
  const layout = h(
    "div",
    { id: "layout", class: ui.sidebarOpen ? "sidebar-open" : "" },
    renderSidebar(),
    h("div", { id: "resizer", role: "separator", "aria-orientation": "vertical", title: "Drag to resize (> / <)", onmousedown: startResize }),
    renderRight(),
  );
  // Re-rendering replaces the panes; keep where they were scrolled to.
  const contentTop = $("#content")?.scrollTop ?? 0;
  const listTop = $("#sections")?.scrollTop ?? 0;
  $("#app").replaceChildren(renderTitleBar(title), layout, renderStatusBar());
  $("#sidebar").style.width = `${ui.leftRatio}%`;
  const content = $("#content");
  if (content) content.scrollTop = contentTop;
  $("#sections").scrollTop = listTop;
  renderModal();
  afterRender();
}

// replaceSectionList re-renders the section list in place, keeping where it
// was scrolled to.
function replaceSectionList() {
  const old = $("#sections");
  const top = old.scrollTop;
  const fresh = renderSectionList();
  old.replaceWith(fresh);
  fresh.scrollTop = top;
  fresh.querySelector(".active")?.scrollIntoView({ block: "nearest" });
}

// visibleTop returns where the visible part of the content pane starts:
// below the sticky section header of the section view, which covers the
// rows scrolled under it.
export function visibleTop(content) {
  const top = content.getBoundingClientRect().top;
  const head = content.querySelector(".block-head");
  return head ? Math.max(head.getBoundingClientRect().bottom, top) : top;
}

// scrollRowIntoView scrolls the content pane as little as needed to show a
// row in its visible part (unlike scrollIntoView, which ignores the header).
function scrollRowIntoView(row) {
  const content = $("#content");
  const r = row.getBoundingClientRect();
  const top = visibleTop(content);
  const bottom = content.getBoundingClientRect().bottom;
  if (r.top < top) content.scrollTop -= top - r.top;
  else if (r.bottom > bottom) content.scrollTop += r.bottom - bottom;
}

// refreshCursor moves the raw view's cursor highlight and updates the list
// and status bar without rebuilding the content, so its scroll is untouched.
export function refreshCursor() {
  for (const row of document.querySelectorAll("#content tr[data-idx]")) {
    row.classList.toggle("cursor", Number(row.dataset.idx) === ui.lineCursor);
  }
  replaceSectionList();
  $("#statusbar").replaceWith(renderStatusBar());
}

// refreshPanes re-renders the section list, the right pane and the status
// bar but not the search input, so typing (including IME composition) in
// it is not interrupted.
export function refreshPanes() {
  const contentTop = $("#content")?.scrollTop ?? 0;
  replaceSectionList();
  $("#right").replaceWith(renderRight());
  $("#statusbar").replaceWith(renderStatusBar());
  $("#content").scrollTop = contentTop;
  afterRender();
}

function renderTitleBar(title) {
  const f = file();
  // Icon-only buttons get their title as accessible name.
  const btn = (label, titleText, onclick, cls = "btn") => button(label, { class: cls, title: titleText, "aria-label": cls.includes("icon") ? titleText : null, onclick });
  return h(
    "header",
    { id: "topbar" },
    // Narrow windows show the section list on demand (there are no panes to focus).
    btn("☰", "Sections", () => guarded(() => (ui.sidebarOpen = !ui.sidebarOpen), true), "btn icon narrow-only"),
    h("span", { class: "doc-title" }, title),
    h("span", { class: "spacer" }),
    h(
      "div",
      { class: "segmented", role: "group", "aria-label": "View" },
      button("Rendered", { "aria-pressed": String(!ui.rawView), title: "Rendered view (r)", onclick: () => guarded(() => ui.rawView && toggleRaw()) }),
      button(f.diff ? "Diff" : "Raw", { "aria-pressed": String(ui.rawView), title: "Raw source view (r)", onclick: () => guarded(() => !ui.rawView && toggleRaw()) }),
    ),
    button("Full view", { class: "btn", "aria-pressed": String(ui.fullView), title: "Full / section view (f)", onclick: () => guarded(toggleFull) }),
    // Only the look changes, so the theme can be switched in every mode.
    btn("◐", "Toggle theme", toggleTheme, "btn icon"),
    btn("?", "Help (?)", () => guarded(openHelp, true), "btn icon"),
    btn(st.multiFile ? "Skip file" : "Quit", "Quit (q)", () => guarded(() => openConfirm("quit"), true)),
    btn(st.multiFile ? "Finish file" : "Submit", "Submit (s)", () => guarded(() => openConfirm("submit"), true), "btn primary"),
  );
}

function renderSidebar() {
  const search =
    ui.mode === "search" || ui.query
      ? h("input", {
          id: "search",
          type: "search",
          value: ui.query,
          placeholder: "Search sections",
          autocomplete: "off",
          "aria-label": "Search sections",
          readonly: ui.mode === "search" ? null : true,
          oninput: (e) => {
            ui.query = e.target.value;
            runSearch();
          },
          onmousedown: (e) => {
            if (ui.mode !== "normal") return;
            // The re-render replaces this input; the default focus would land
            // on the removed element, so afterRender focuses the new one.
            e.preventDefault();
            openSearch();
            render();
          },
        })
      : null;
  return h("aside", { id: "sidebar" }, search, renderSectionList());
}

function renderSectionList() {
  const parents = parentIds();
  const items = listSections().map((s) => {
    const n = commentsOf(s.id).length;
    const viewed = isViewed(s.id);
    const active = s.id === ui.cursor;
    const expandable = parents.has(s.id);
    return h(
      "li",
      {
        class: `item${active ? " active" : ""}${viewed ? " viewed" : ""}`,
        "aria-current": active ? "true" : null,
        style: `padding-left:${4 + s.depth * 14}px`,
        // During a search, a click picks the result and confirms (Enter).
        onclick: () =>
          guarded(() => {
            ui.sidebarOpen = false; // narrow windows: back to the content
            moveCursorTo(s.id);
          }, true),
      },
      h(
        "span",
        {
          class: "twisty",
          onclick: (e) => {
            if (!expandable) return;
            e.stopPropagation();
            guarded(() => {
              ui.cursor = s.id;
              toggleExpand();
            });
          },
        },
        expandable ? (ui.collapsed.has(s.id) ? "▶" : "▼") : "",
      ),
      sidBadge(s.id),
      h("span", { class: "title", title: s.title }, s.title),
      n ? h("span", { class: "badge", title: plural(n, "comment") }, n) : null,
      h("span", { class: "check" }, viewed ? "✓" : ""),
    );
  });
  if (!items.length) items.push(h("li", { class: "empty" }, ui.query ? "No matches" : "No sections"));
  return h("ul", { id: "sections", "aria-label": "Sections" }, items);
}

function renderRight() {
  const main = h("main", { id: "content" });
  if (ui.mode === "commentList") {
    main.append(renderCommentList());
    return h("div", { id: "right" }, main);
  }
  const f = file();
  if (ui.rawView) {
    main.append(renderRawBlock());
  } else if (!f.sections.length) {
    main.append(h("div", { class: "empty" }, "This document is empty."));
  } else {
    if (ui.fullView) main.append(renderFullDocument());
    else main.append(...f.sections.filter((s) => s.id === ui.cursor).map(renderRenderedBlock));
  }
  const right = h("div", { id: "right" }, main);
  if (ui.mode === "comment") right.append(renderEditor());
  return right;
}

function blockHead(s) {
  const viewed = isViewed(s.id);
  return h(
    "div",
    { class: "block-head" },
    sidBadge(s.id),
    h("span", { class: "stitle" }, s.title),
    isRealSection(s.id)
      ? h(
          "label",
          { title: "Viewed (v)" },
          h("input", { type: "checkbox", checked: viewed, onclick: (e) => (e.preventDefault(), guarded(() => toggleViewed(s.id))) }),
          "Viewed",
        )
      : null,
    // Section comments are made from the rendered view, as in the TUI.
    ui.rawView ? null : button("Comment", { class: "btn small", title: "Comment on this section (c)", onclick: () => guarded(() => openSectionEditor(s.id)) }),
  );
}

// sidBadge shows a section's ID; the overview has none.
function sidBadge(id) {
  return isRealSection(id) ? h("span", { class: "sid" }, id) : null;
}

function renderRenderedBlock(s) {
  const body = h("div", { class: "markdown" });
  body.innerHTML = s.html; // rendered by goldmark with raw HTML escaped
  return h(
    "section",
    { class: `block${isViewed(s.id) ? " viewed" : ""}${s.id === ui.cursor ? " current" : ""}`, dataset: { section: s.id } },
    blockHead(s),
    body,
    commentsBox(commentsOf(s.id), true),
  );
}

// renderFullDocument renders the whole document as one Markdown page with no
// section chrome; each section's comments follow its content, as in the
// TUI's full view. The data-section parts let the list follow the scroll.
function renderFullDocument() {
  return h(
    "article",
    { class: "document markdown" },
    file().sections.map((s) => {
      const part = h("div", { class: "part", dataset: { section: s.id } });
      part.innerHTML = s.html; // rendered by goldmark with raw HTML escaped
      const comments = commentsBox(commentsOf(s.id), true);
      if (comments) part.append(comments);
      return part;
    }),
  );
}

// renderRawBlock renders the raw view: one continuous listing in the full
// view, like the TUI's, or the selected section's lines under its header.
function renderRawBlock() {
  const vis = visibleLines();
  const s = section(ui.cursor);
  if (!vis.length) {
    const msg = file().diff ? "No changes in this section." : "No lines.";
    return h("section", { class: "block" }, s ? blockHead(s) : null, h("div", { class: "empty" }, msg));
  }
  // In the full view, section comments follow the last line of their section.
  if (ui.fullView) return h("section", { class: "block document" }, renderLines(vis, true));
  const sectionComments = s ? commentsOf(s.id).filter((c) => !c.startLine) : [];
  return h(
    "section",
    { class: `block${s && isViewed(s.id) ? " viewed" : ""}`, dataset: { section: ui.cursor } },
    s ? blockHead(s) : null,
    renderLines(vis),
    commentsBox(sectionComments, false),
  );
}

function renderLines(indices, withSectionComments = false) {
  const f = file();
  const byEnd = new Map();
  for (const c of f.comments) {
    if (!c.startLine) continue;
    const key = `${c.side || ""}:${c.endLine || c.startLine}`;
    if (!byEnd.has(key)) byEnd.set(key, []);
    byEnd.get(key).push(c);
  }
  const range = ui.mode === "lineSelect" ? selectionRange() : null;
  const rows = [];
  let lastNew = 0; // new-file line of the previous added/context line
  indices.forEach((i, pos) => {
    const l = f.lines[i];
    if (f.diff && l.side === "RIGHT") {
      if (lastNew && l.line !== lastNew + 1) rows.push(h("tr", { class: "gap" }, h("td", { colspan: "3" })));
      lastNew = l.line;
    }
    const kind = l.type === "+" ? "add" : l.type === "-" ? "del" : "";
    const cls = [kind, inSelection(range, i) ? "sel" : "", i === ui.lineCursor ? "cursor" : ""].filter(Boolean).join(" ");
    rows.push(
      h(
        "tr",
        { class: cls, dataset: { idx: i } },
        h("td", { class: "num", dataset: { idx: i }, title: "Click to comment; drag or shift+click for a range" }, l.line),
        h("td", { class: "mark" }, l.type || ""),
        h("td", { class: "code" }, l.text),
      ),
    );
    const inline = byEnd.get(`${l.side || ""}:${l.line}`);
    const next = f.lines[indices[pos + 1]];
    const endOfSection = withSectionComments && (!next || next.section !== l.section);
    const sectionComments = endOfSection ? commentsOf(l.section).filter((c) => !c.startLine) : [];
    const boxes = [...(inline || []), ...sectionComments];
    if (boxes.length) rows.push(h("tr", { class: "inline" }, h("td", { colspan: "3" }, boxes.map((c) => renderComment(c, false)))));
  });
  return h("table", { class: "lines" }, h("tbody", {}, rows));
}

// commentsBox renders a list of comments, or nothing when there are none.
function commentsBox(comments, withRef) {
  return comments.length ? h("div", { class: "comments" }, comments.map((c) => renderComment(c, withRef))) : null;
}

function renderComment(c, withRef) {
  return h(
    "div",
    { class: "comment" },
    h(
      "div",
      { class: "comment-head" },
      h("span", { class: `label label-${c.action}` }, c.action),
      c.decoration ? h("span", { class: "deco" }, `(${c.decoration})`) : null,
      withRef && c.outputRef ? h("span", { class: "ref" }, c.outputRef) : null,
    ),
    withRef && c.quote && c.quote.length ? h("div", { class: "quote" }, c.quote.join("\n")) : null,
    h("div", { class: "comment-body" }, c.body),
  );
}

function renderCommentList() {
  const list = ui.list;
  const comments = commentsOf(list.sectionId);
  return h(
    "div",
    { class: "comment-list" },
    h("h2", {}, `Comments on ${list.sectionId}`),
    comments.map((c, i) =>
      h(
        "div",
        {
          class: `list-item${i === list.cursor ? " active" : ""}`,
          onclick: () => {
            list.cursor = i;
            render();
          },
        },
        h(
          "div",
          { class: "list-head" },
          `${i === list.cursor ? "> " : "  "}#${i + 1} [${c.label}]${c.ref ? ` (${c.ref})` : ""}`,
          h(
            "span",
            { class: "actions" },
            button("Edit (e)", { class: "btn small", onclick: (e) => (e.stopPropagation(), (list.cursor = i), editFromList()) }),
            button("Delete (d)", { class: "btn small danger", onclick: (e) => (e.stopPropagation(), (list.cursor = i), deleteFromList()) }),
          ),
        ),
        h("div", { class: "list-body" }, c.body.split("\n")[0]),
      ),
    ),
  );
}

function renderEditor() {
  const e = ui.editor;
  const chips = (values, current, pick, show) =>
    h(
      "div",
      { class: "chips" },
      values.map((v, i) =>
        button(show(v), {
          class: `chip label-${v || "none"}`,
          "aria-pressed": String(i === current),
          tabindex: "-1",
          onmousedown: (ev) => ev.preventDefault(), // keep the textarea focused
          onclick: () => {
            pick(i);
            updateEditorChrome();
          },
        }),
      ),
    );
  const textarea = h("textarea", { id: "editor-body", placeholder: "Enter review comment... (Ctrl+S to save, Esc to cancel)", "aria-label": "Comment body" });
  textarea.value = e.body;
  textarea.addEventListener("input", () => {
    e.body = textarea.value;
  });
  const ref = e.startLine ? ` (${lineRef(e)})` : "";
  return h(
    "div",
    { id: "editor", class: "editor" },
    h("div", { class: "editor-sep" }, `Comment [${formatLabel(st.labels[e.label], st.decorations[e.deco])}]${ref}`),
    chips(st.labels, e.label, (i) => (e.label = i), (v) => v),
    chips(st.decorations, e.deco, (i) => (e.deco = i), (v) => v || "none"),
    textarea,
  );
}

function renderStatusBar() {
  const entries = statusKeys().map(([key, label]) => h("span", { class: "entry" }, h("kbd", {}, key), " ", label));
  if (ui.mode === "lineSelect") entries.unshift(h("span", { class: "visual" }, "VISUAL"));
  return h("footer", { id: "statusbar" }, h("span", { class: "entries" }, entries), h("span", { class: "indicator" }, statusIndicator()));
}

function renderModal() {
  const root = $("#modal-root");
  if (ui.mode === "confirm") {
    root.replaceChildren(
      h(
        "div",
        { class: "backdrop" },
        h(
          "div",
          { class: "modal", role: "dialog", "aria-modal": "true" },
          h("p", { class: "message" }, confirmMessage()),
          h(
            "div",
            { class: "buttons" },
            button([h("kbd", {}, "y"), " yes"], { class: "btn primary", onclick: executeConfirm }),
            button([h("kbd", {}, "n"), " no"], { class: "btn", onclick: closeModal }),
            button([h("kbd", {}, "esc"), " cancel"], { class: "btn", onclick: closeModal }),
          ),
        ),
      ),
    );
  } else if (ui.mode === "help") {
    root.replaceChildren(
      h(
        "div",
        { class: "backdrop", onclick: (e) => e.target === e.currentTarget && closeModal() },
        h("div", { class: "modal help", role: "dialog", "aria-modal": "true" }, h("h2", {}, "commd - Help"), h("pre", {}, helpText())),
      ),
    );
  } else {
    root.replaceChildren();
  }
}

// afterRender restores focus and scroll positions that a re-render loses.
function afterRender() {
  if (ui.mode === "comment") {
    const ta = $("#editor-body");
    if (ta && document.activeElement !== ta) {
      ta.focus();
      ta.setSelectionRange(ta.value.length, ta.value.length);
    }
  } else if (ui.mode === "search") {
    const input = $("#search");
    if (input && document.activeElement !== input) {
      input.focus();
      input.setSelectionRange(input.value.length, input.value.length);
    }
  }
  $("#sections .active")?.scrollIntoView({ block: "nearest" });
  const row = ui.rawView ? $("#content tr.cursor") : null;
  if (row) scrollRowIntoView(row);
  if (ui.pendingScroll) {
    ui.pendingScroll();
    ui.pendingScroll = null;
  }
}

// updateEditorChrome refreshes the editor's label line and chips while
// keeping the textarea itself, so the caret and any IME input stay put.
export function updateEditorChrome() {
  const old = $("#editor");
  const ta = $("#editor-body");
  if (!old || !ta) return;
  const fresh = renderEditor();
  fresh.querySelector("#editor-body").replaceWith(ta);
  old.replaceWith(fresh);
  ta.focus();
  $("#statusbar").replaceWith(renderStatusBar());
}
