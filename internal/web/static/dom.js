// Small DOM helpers.

import { ui } from "./state.js";

export const $ = (sel) => document.querySelector(sel);

// h builds an element. Children that are not Nodes become text, so
// document text is never parsed as HTML.
export function h(tag, attrs, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v === false || v == null) continue;
    if (k === "class") el.className = v;
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else if (k === "dataset") Object.assign(el.dataset, v);
    else el.setAttribute(k, v === true ? "" : v);
  }
  for (const c of children.flat(Infinity)) {
    if (c == null || c === false) continue;
    el.append(c instanceof Node ? c : String(c));
  }
  return el;
}

export const plural = (n, word) => `${n} ${word}${n === 1 ? "" : "s"}`;

let toastTimer = 0;
export function toast(msg) {
  if (ui.finished) return;
  const el = $("#toast");
  el.textContent = msg;
  el.classList.add("show");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.remove("show"), 4000);
}
