// The file picker (phase "pick"), like the TUI's.

import { $, h, button } from "./dom.js";
import { st, ui } from "./state.js";
import { togglePick, confirmPick, cancelPick } from "./actions.js";

export function renderPicker() {
  document.title = "commd — select files";
  const p = ui.picker;
  $("#app").replaceChildren(
    h(
      "div",
      { class: "picker" },
      h("h1", {}, "Select Markdown files to review"),
      h("hr"),
      h(
        "ul",
        {},
        st.pick.map((path, i) =>
          h(
            "li",
            {
              class: i === p.cursor ? "active" : "",
              onclick: () => {
                p.cursor = i;
                togglePick(i);
              },
            },
            `${i === p.cursor ? "▸ " : "  "}${p.selected.has(i) ? "[✓]" : "[ ]"} ${path}`,
          ),
        ),
      ),
      h("p", { class: "hint" }, "↑/↓ navigate • space toggle • a all • enter confirm • q cancel"),
      h("div", { class: "buttons" }, button("Cancel", { class: "btn", onclick: cancelPick }), button("Confirm", { class: "btn primary", onclick: confirmPick })),
    ),
  );
  $("#modal-root").replaceChildren();
}
