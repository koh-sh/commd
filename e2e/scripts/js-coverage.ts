// Summarizes the JS coverage of the web UI collected by the browser tests
// with COMMD_JS_COVERAGE=1 (see helpers/browser.ts): the share of each
// script's code that ran in at least one test. Run via `mise run e2e-cov`.
import { readdirSync, readFileSync } from "fs";
import { join } from "path";
import { COVERAGE_DIR } from "../helpers/browser";

interface Range {
  startOffset: number;
  endOffset: number;
  count: number;
}
interface Entry {
  url: string;
  source?: string;
  functions: { ranges: Range[] }[];
}

// covered[file][offset] is true when the byte ran in some test.
const covered = new Map<string, { source: string; ran: Uint8Array }>();

for (const name of readdirSync(COVERAGE_DIR)) {
  const entries: Entry[] = JSON.parse(readFileSync(join(COVERAGE_DIR, name), "utf8"));
  for (const e of entries) {
    if (!e.source) continue;
    const file = new URL(e.url).pathname.slice(1);
    // V8 block ranges nest; applying them from the largest to the smallest
    // leaves each byte with its innermost count.
    const counts = new Int32Array(e.source.length).fill(-1);
    const ranges = e.functions.flatMap((f) => f.ranges).sort((a, b) => b.endOffset - b.startOffset - (a.endOffset - a.startOffset));
    for (const r of ranges) counts.fill(r.count, r.startOffset, r.endOffset);
    const acc = covered.get(file) ?? { source: e.source, ran: new Uint8Array(e.source.length) };
    for (let i = 0; i < counts.length; i++) if (counts[i] > 0) acc.ran[i] = 1;
    covered.set(file, acc);
  }
}

// Report per file, counting only non-whitespace characters.
let total = 0;
let hit = 0;
const rows: string[] = [];
for (const [file, { source, ran }] of [...covered].sort()) {
  let t = 0;
  let c = 0;
  for (let i = 0; i < source.length; i++) {
    if (/\s/.test(source[i])) continue;
    t++;
    if (ran[i]) c++;
  }
  total += t;
  hit += c;
  rows.push(`${file.padEnd(12)} ${((100 * c) / t).toFixed(1).padStart(5)}%`);
}
console.log(rows.join("\n"));
console.log(`${"total".padEnd(12)} ${((100 * hit) / total).toFixed(1).padStart(5)}%`);

// With --uncovered, list the lines that have code that never ran.
if (process.argv.includes("--uncovered")) {
  for (const [file, { source, ran }] of [...covered].sort()) {
    let offset = 0;
    source.split("\n").forEach((line, i) => {
      let missed = false;
      for (let j = 0; j < line.length; j++) if (!/\s/.test(line[j]) && !ran[offset + j]) missed = true;
      if (missed) console.log(`${file}:${i + 1}: ${line.trim()}`);
      offset += line.length + 1;
    });
  }
}
