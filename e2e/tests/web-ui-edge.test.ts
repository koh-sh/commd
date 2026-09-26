import { describe, test, expect, afterEach } from "bun:test";
import { writeFileSync, rmSync } from "fs";
import { resolve } from "path";
import type { Page } from "playwright";
import { TEST_TIMEOUT, FIXTURE_BASIC } from "../helpers/session";
import { createRepo, createRepoFrom } from "../helpers/git-repo";
import { launchWeb, finished, stopWeb, type WebSession } from "../helpers/web";
import { COMMD_BIN, PROJECT_ROOT } from "../helpers/paths";
import {
  useBrowser, openPage, closePage, press, eventually, consistently, text, count, activeSection, cursorLine, writeFixture,
} from "../helpers/browser";

// Full suite: session edge cases (the command going away, a second tab),
// layouts, unusual documents, input during slow requests, and the
// browser-only mouse extras.

useBrowser();

const stateOf = async (web: WebSession) => (await web.api("GET", "/api/state")).json();

describe("Web Review UI Edge Cases (Full)", () => {
  let web: WebSession | undefined;
  let pages: Page[] = [];
  let repo: { dir: string; cleanup: () => void } | undefined;
  let fixture: { path: string; cleanup: () => void } | undefined;

  async function open(w: WebSession, opts?: { width?: number; height?: number }): Promise<Page> {
    const p = await openPage(w, opts);
    pages.push(p);
    return p;
  }

  afterEach(async () => {
    for (const p of pages) await closePage(p);
    pages = [];
    await stopWeb(web);
    web = undefined;
    repo?.cleanup();
    repo = undefined;
    fixture?.cleanup();
    fixture = undefined;
  });

  test(
    "the page ends when commd is stopped",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      const page = await open(web);
      await stopWeb(web);
      await press(page, "j", "v");
      await eventually(async () => expect(await text(page, ".done")).toContain("Review session ended"));
      // Nothing reacts any more.
      await press(page, "q");
      expect(await count(page, ".modal")).toBe(0);
    },
    TEST_TIMEOUT,
  );

  test(
    "a second tab catches up when the first finishes a file",
    async () => {
      repo = createRepo(true);
      web = await launchWeb(["--diff", "doc.md", "new.md"], repo.dir);
      const a = await open(web);
      const b = await open(web);
      await eventually(async () => expect(await text(b, ".doc-title")).toContain("(doc.md)"));

      await press(a, "s", "y");
      await eventually(async () => expect(await text(a, ".doc-title")).toContain("(new.md)"));
      // b still shows doc.md; its next change is rejected and it reloads.
      await b.locator("#sections .item").nth(1).click();
      await press(b, "v");
      await eventually(async () => expect(await text(b, ".doc-title")).toContain("(new.md)"));
      await eventually(async () => expect(await text(b, "#toast")).toContain("no longer under review"));

      await press(a, "q", "y");
      await eventually(async () => expect(await text(a, ".done")).toContain("Review finished"));
      await press(b, "s", "y"); // b's session is over too
      await eventually(async () => expect(await count(b, ".done")).toBe(1));
      expect((await finished(web)).stderr).toContain("Approved.");
    },
    TEST_TIMEOUT,
  );

  test(
    "narrow windows show the content, with the section list on demand",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      const page = await open(web, { width: 600, height: 700 });
      await eventually(async () => expect(await page.locator("#right").isVisible()).toBe(true));
      expect(await page.locator("#sidebar").isVisible()).toBe(false);
      // The ☰ button opens the list; picking a section goes back to the content.
      await page.getByRole("button", { name: "Sections" }).click();
      await eventually(async () => expect(await page.locator("#sidebar").isVisible()).toBe(true));
      expect(await page.locator("#right").isVisible()).toBe(false);
      await page.locator("#sections .item").nth(3).click();
      await eventually(async () => expect(await page.locator("#right").isVisible()).toBe(true));
      expect(await activeSection(page)).toContain("1.2 Middleware Registration");
      // Search opens the list too.
      await press(page, "/");
      await eventually(async () => expect(await page.locator("#sidebar").isVisible()).toBe(true));
      await press(page, "Escape");
      await eventually(async () => expect(await page.locator("#sidebar").isVisible()).toBe(false));
      // Resizing is off, as in the TUI below 80 columns.
      const width = await page.locator("#sidebar").evaluate((el) => (el as HTMLElement).style.width);
      await press(page, ">");
      expect(await page.locator("#sidebar").evaluate((el) => (el as HTMLElement).style.width)).toBe(width);
    },
    TEST_TIMEOUT,
  );

  test(
    "unusual documents: empty, title only, lines before the first heading",
    async () => {
      fixture = writeFixture("");
      web = await launchWeb([fixture.path]);
      let page = await open(web);
      await eventually(async () => expect(await text(page, "#content")).toContain("This document is empty."));
      await press(page, "c", "C", "v", "r"); // nothing to act on
      await eventually(async () => expect(await count(page, "#editor")).toBe(0));
      await stopWeb(web);
      fixture.cleanup();

      // Without a preamble there is no overview entry (as in the TUI), but
      // the title line still shows in the full raw view and takes comments.
      fixture = writeFixture("# Title\n\n## A\n\nbody\n");
      web = await launchWeb([fixture.path]);
      page = await open(web);
      await eventually(async () => expect(await count(page, "#sections .item")).toBe(1));
      await press(page, "r", "f", "g", "g");
      await eventually(async () => expect(await cursorLine(page)).toBe("1"));
      expect(await count(page, "#content .block:not(:has(.block-head))")).toBe(1);
      await press(page, "c");
      await page.keyboard.type("title");
      await press(page, "Control+s");
      await eventually(async () => expect((await stateOf(web!)).file.comments[0]).toMatchObject({ sectionId: "overview", startLine: 1 }));

      // Back to the section view: only section A's lines.
      await press(page, "f");
      await eventually(async () => expect(await count(page, "#content tr[data-idx]")).toBe(3));
    },
    TEST_TIMEOUT,
  );

  test(
    "unchanged diff sections say so and cannot be commented",
    async () => {
      repo = createRepo(true);
      web = await launchWeb(["--diff", "doc.md"], repo.dir);
      const page = await open(web);
      // Step 1 has no changes.
      await page.locator("#sections .item").nth(1).click();
      await eventually(async () => expect(await text(page, "#content")).toContain("No changes in this section."));
      await press(page, "c", "V");
      await eventually(async () => expect(await count(page, "#editor")).toBe(0));
      expect(await text(page, "#statusbar")).not.toContain("VISUAL");
      // Rendered view of a diff file.
      await press(page, "r");
      await eventually(async () => expect(await text(page, "#content .markdown")).toContain("Alpha body line."));
    },
    TEST_TIMEOUT,
  );

  test(
    "keys and clicks during a slow request are applied in order afterwards",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      const page = await open(web);
      await page.route("**/api/files/**", async (route) => {
        await Bun.sleep(300);
        await route.continue();
      });

      await press(page, "j", "c");
      await page.keyboard.type("first");
      // Saved slowly; the next comment is typed while the save is in flight.
      await press(page, "Control+s", "j", "c");
      await page.keyboard.type("second\nline");
      await press(page, "Backspace", "Control+s");
      await eventually(async () => expect((await stateOf(web!)).file.comments.map((c: { body: string }) => c.body)).toEqual(["first", "second\nlin"]));
      expect(await activeSection(page)).toContain("1.1 JWT Verification");

      // A click while the viewed request is in flight runs after it.
      await press(page, "v");
      await page.locator("#sections .item").nth(4).click();
      await eventually(async () => expect(await activeSection(page)).toContain("Step 2"));
      expect((await stateOf(web)).file.viewed).toEqual(["S1.1"]);
    },
    TEST_TIMEOUT,
  );

  test(
    "mouse extras: view buttons, viewed checkbox, comment button, shift+click, resize, theme",
    async () => {
      web = await launchWeb([FIXTURE_BASIC, "--theme", "light"]);
      const page = await open(web);
      await eventually(async () => expect(await page.evaluate(() => document.documentElement.dataset.theme)).toBe("light"));
      await page.getByRole("button", { name: "Toggle theme" }).click();
      expect(await page.evaluate(() => document.documentElement.dataset.theme)).toBe("dark");

      // Viewed checkbox; Space afterwards must not toggle it again.
      await page.locator("#sections .item").nth(1).click();
      await page.locator(".block-head input[type=checkbox]").click();
      await eventually(async () => expect((await stateOf(web!)).file.viewed).toEqual(["S1"]));
      await press(page, " ");
      await consistently(async () => expect((await stateOf(web!)).file.viewed).toEqual(["S1"]));

      // Comment button; the Full view and Raw buttons.
      await page.locator(".block-head button", { hasText: "Comment" }).click();
      await page.keyboard.type("button");
      await press(page, "Control+s");
      await page.getByRole("button", { name: "Full view" }).click();
      await eventually(async () => expect(await count(page, "#content .part")).toBe(8));
      await page.getByRole("button", { name: "Raw" }).click();
      await eventually(async () => expect(await count(page, "#content tr[data-idx]")).toBe(31));
      // Section comments are made from the rendered view only.
      expect(await count(page, ".block-head button")).toBe(0);

      // Click then shift+click selects a range.
      await page.locator("#content td.num").nth(4).click();
      await eventually(async () => expect(await text(page, ".editor-sep")).toBe("Comment [question] (L5)"));
      await press(page, "Escape");
      await page.locator("#content td.num").nth(6).click({ modifiers: ["Shift"] });
      await eventually(async () => expect(await text(page, ".editor-sep")).toBe("Comment [question] (L5-L7)"));
      await press(page, "Escape");
      await page.getByRole("button", { name: "Rendered" }).click();
      await eventually(async () => expect(await count(page, "#content .part")).toBe(8));

      // Dragging the border resizes the left pane.
      const box = (await page.locator("#resizer").boundingBox())!;
      await page.mouse.move(box.x + 2, box.y + 100);
      await page.mouse.down();
      await page.mouse.move(640, box.y + 100);
      await page.mouse.up();
      expect(await page.locator("#sidebar").evaluate((el) => (el as HTMLElement).style.width)).toBe("50%");

      // Help button.
      await page.getByRole("button", { name: "Help (?)" }).click();
      await eventually(async () => expect(await count(page, ".modal.help")).toBe(1));
      await press(page, "Escape");

      await page.getByRole("button", { name: "Submit" }).click();
      await page.locator(".modal button", { hasText: "yes" }).click();
      const { stdout } = await finished(web);
      expect(stdout).toContain("[question] button");
    },
    TEST_TIMEOUT,
  );
});

describe("Web Review UI Errors and Remaining Paths (Full)", () => {
  let web: WebSession | undefined;
  let page: Page | undefined;
  let repo: { dir: string; cleanup: () => void } | undefined;

  afterEach(async () => {
    await closePage(page);
    page = undefined;
    await stopWeb(web);
    web = undefined;
    repo?.cleanup();
    repo = undefined;
  });

  test(
    "a --port already in use fails to start",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      const port = new URL(web.base).port;
      const second = Bun.spawnSync([COMMD_BIN, "review", FIXTURE_BASIC, "--web", "--no-open", "--port", port], {
        cwd: PROJECT_ROOT,
        stdin: "ignore",
      });
      expect(second.exitCode).not.toBe(0);
      expect(second.stderr.toString()).toContain("starting web server");
    },
    TEST_TIMEOUT,
  );

  test(
    "API errors show a toast; a failed or refused state load ends the page",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      page = await openPage(web);
      await page.route("**/api/files/*/comments", (route) => route.fulfill({ status: 400, contentType: "application/json", body: '{"error":"boom"}' }));
      await press(page, "c");
      await page.keyboard.type("x");
      await press(page, "Control+s");
      await eventually(async () => expect(await text(page!, "#toast")).toBe("boom"));
      // The editor stays open so the comment is not lost.
      expect(await count(page, "#editor")).toBe(1);
      await page.unroute("**/api/files/*/comments");
      await page.route("**/api/files/*/comments", (route) => route.fulfill({ status: 502, body: "bad gateway" }));
      await press(page, "Control+s");
      await eventually(async () => expect(await text(page!, "#toast")).toContain("502"));
      await press(page, "Escape");

      // A wrong token cannot load the review.
      const other = await openPage({ ...web, url: web.url.replace(/token=\w+/, "token=wrong") });
      await eventually(async () => expect(await text(other, ".done")).toContain("Cannot load the review"));
      await closePage(other);

      // A state load that fails (commd gone) ends the page.
      const gone = await (await import("playwright")).chromium.launch();
      const p = await gone.newPage();
      await p.route("**/api/state", (route) => route.abort());
      await p.goto(web.url);
      await eventually(async () => expect(await p.locator(".done").innerText()).toContain("Review session ended"));
      await gone.close();
    },
    TEST_TIMEOUT,
  );

  test(
    "line comments show their reference and quote in the rendered view; hunks are separated",
    async () => {
      // One section with changes at both ends: two hunks, separated in the view.
      const body = Array.from({ length: 20 }, (_, i) => `line `);
      const changed = [...body];
      changed[0] = "line one";
      changed[19] = "line twenty";
      const gapRepo = createRepoFrom({ "gap.md": ["## Only", "", ...body, ""].join("\n") }, { "gap.md": ["## Only", "", ...changed, ""].join("\n") });
      try {
        const gapWeb = await launchWeb(["--diff", "gap.md"], gapRepo.dir);
        const gapPage = await openPage(gapWeb);
        await eventually(async () => expect(await count(gapPage, "#content tr.gap")).toBe(1));
        await closePage(gapPage);
        await stopWeb(gapWeb);
      } finally {
        gapRepo.cleanup();
      }

      repo = createRepo(true);
      web = await launchWeb(["--diff", "doc.md"], repo.dir);
      page = await openPage(web);
      await press(page, "c"); // removed title line
      await page.keyboard.type("old");
      await press(page, "Control+s", "r");
      await eventually(async () => expect(await text(page!, "#content .comment .ref")).toBe("L1 (removed)"));
      expect(await text(page, "#content .comment .quote")).toBe("# Diff Doc");
    },
    TEST_TIMEOUT,
  );

  test(
    "input the TUI would not accept in the current mode is refused or ignored",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      page = await openPage(web);

      // Tree toggle by clicking the triangle.
      await page.locator("#sections .item").nth(1).locator(".twisty").click();
      await eventually(async () => expect(await count(page!, "#sections .item")).toBe(6));
      await page.locator("#sections .item").nth(0).locator(".twisty").dispatchEvent("click"); // no children: nothing
      expect(await count(page, "#sections .item")).toBe(6);

      // Clicks during the comment list are refused with a hint.
      await press(page, "c");
      await page.keyboard.type("x");
      await press(page, "Control+s", "C");
      await page.getByRole("button", { name: "Full view" }).click();
      await eventually(async () => expect(await text(page!, "#toast")).toContain("Finish the current action first"));
      await press(page, "Escape");

      // A line-number click while commenting is refused.
      await press(page, "r", "c");
      await page.locator("#content td.num").first().dispatchEvent("mousedown", { button: 0 });
      await eventually(async () => expect(await text(page!, "#toast")).toContain("Save (Ctrl+S)"));
      await press(page, "Escape");

      // Visual selection upwards from the last line (G), then Esc.
      await press(page, "G", "V", "k", "ArrowUp");
      await eventually(async () => expect(await text(page!, "#statusbar .indicator")).toBe("L29-L31"));
      await press(page, "Escape");

      // Keys that are part of an IME composition, or use Alt/Meta, are not shortcuts.
      await page.evaluate(() => document.dispatchEvent(new KeyboardEvent("keydown", { key: "q", isComposing: true, bubbles: true })));
      await page.evaluate(() => document.dispatchEvent(new KeyboardEvent("keydown", { key: "q", altKey: true, bubbles: true })));
      await page.evaluate(() => document.dispatchEvent(new KeyboardEvent("keydown", { key: "q", metaKey: true, bubbles: true })));
      expect(await count(page, ".modal")).toBe(0);

      // Scrolling the full view with the wheel resumes following the scroll.
      await press(page, "r", "f", "G");
      await page.locator("#content").hover();
      await page.mouse.wheel(0, -5000);
      await eventually(async () => expect(await activeSection(page!)).toContain("Overview"));
    },
    TEST_TIMEOUT,
  );

  test(
    "picker ignores other keys and copies with a text selection",
    async () => {
      repo = createRepo(true);
      web = await launchWeb(["--diff"], repo.dir);
      page = await openPage(web);
      await eventually(async () => expect(await count(page!, ".picker")).toBe(1));
      await press(page, "x");
      await page.locator(".picker h1").selectText();
      await press(page, "Control+c");
      await consistently(() => expect(web!.proc.exitCode).toBeNull());
      await page.evaluate(() => window.getSelection()?.removeAllRanges());
      await press(page, "Control+c");
      expect((await finished(web)).stdout).toBe("");
    },
    TEST_TIMEOUT,
  );
});

describe("Web Review UI Documents and Reload (Full)", () => {
  let web: WebSession | undefined;
  let page: Page | undefined;
  const cleanups: (() => void)[] = [];

  afterEach(async () => {
    await closePage(page);
    page = undefined;
    await stopWeb(web);
    web = undefined;
    for (const c of cleanups.splice(0)) c();
  });

  test(
    "relative images in the document are shown",
    async () => {
      // A 1x1 PNG next to the document, referenced by a relative path.
      const png = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==", "base64");
      const imgName = `.tmp-img-${Date.now()}.png`;
      const imgPath = resolve(import.meta.dir, imgName);
      writeFileSync(imgPath, png);
      cleanups.push(() => rmSync(imgPath, { force: true }));
      const fixture = writeFixture(`## Picture\n\n![dot](${imgName})\n`);
      cleanups.push(fixture.cleanup);

      web = await launchWeb([fixture.path]);
      page = await openPage(web);
      await eventually(async () =>
        expect(await page!.locator("#content img").evaluate((img) => (img as HTMLImageElement).naturalWidth)).toBe(1),
      );
    },
    TEST_TIMEOUT,
  );

  test(
    "reloading asks nothing and keeps the review",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      page = await openPage(web);
      let dialogs = 0;
      page.on("dialog", () => dialogs++);

      await press(page, "c");
      await page.keyboard.type("kept");
      await press(page, "Control+s");
      await eventually(async () => expect(await count(page!, "#content .comment")).toBe(1));
      await page.reload();
      await eventually(async () => expect(await text(page!, "#content .comment")).toContain("kept"));
      expect(dialogs).toBe(0);
    },
    TEST_TIMEOUT,
  );
});
