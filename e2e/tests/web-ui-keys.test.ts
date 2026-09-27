import { describe, test, expect, afterEach } from "bun:test";
import type { Page } from "playwright";
import { TEST_TIMEOUT, FIXTURE_BASIC } from "../helpers/session";
import { launchWeb, finished, stopWeb, type WebSession } from "../helpers/web";
import {
  useBrowser, openPage, closePage, press, eventually, consistently, text, count, activeSection, cursorLine, writeFixture,
} from "../helpers/browser";

// Full suite: navigation and view keys of the web page. Unlike the TUI there
// is no pane focus: vertical keys move the content (or the raw line cursor)
// and step into the next or previous section at its edges.

useBrowser();

// A document with a wide code block (horizontal scrolling) and a long
// section (vertical scrolling), between short ones.
const WIDE_DOC = [
  "# Wide",
  "",
  "Intro.",
  "",
  "## Code",
  "",
  "```text",
  "x".repeat(600),
  "```",
  "",
  "## Long",
  "",
  ...Array.from({ length: 120 }, (_, i) => `Paragraph line ${i + 1}.\n`),
  "## Last",
  "",
  "The end.",
  "",
].join("\n");

const contentScroll = (page: Page) => page.locator("#content").evaluate((el) => el.scrollTop);
const atBottom = (page: Page) => page.locator("#content").evaluate((el) => el.scrollTop + el.clientHeight >= el.scrollHeight - 1);
const preScroll = (page: Page) => page.locator("#content pre").first().evaluate((el) => el.scrollLeft);

describe("Web Review UI Keys (Full)", () => {
  let web: WebSession | undefined;
  let page: Page | undefined;
  let fixture: { path: string; cleanup: () => void } | undefined;

  afterEach(async () => {
    await closePage(page);
    page = undefined;
    await stopWeb(web);
    web = undefined;
    fixture?.cleanup();
    fixture = undefined;
  });

  test(
    "j/k and arrows scroll the section and step to the next or previous one at its edges",
    async () => {
      fixture = writeFixture(WIDE_DOC);
      web = await launchWeb({ file: fixture.path });
      page = await openPage(web);

      // Short sections fit: every press steps on.
      await press(page, "j");
      await eventually(async () => expect(await activeSection(page!)).toContain("Code"));
      await press(page, "ArrowDown");
      await eventually(async () => expect(await activeSection(page!)).toContain("Long"));

      // A long section scrolls first.
      await press(page, "j");
      await eventually(async () => expect(await contentScroll(page!)).toBe(20));
      expect(await activeSection(page)).toContain("Long");
      await press(page, "ArrowUp");
      await eventually(async () => expect(await contentScroll(page!)).toBe(0));

      // At its end, the next press steps on; stepping back shows the end.
      await page.locator("#content").evaluate((el) => (el.scrollTop = el.scrollHeight));
      await eventually(async () => expect(await atBottom(page!)).toBe(true));
      await press(page, "j");
      await eventually(async () => expect(await activeSection(page!)).toContain("Last"));
      await press(page, "k");
      await eventually(async () => expect(await activeSection(page!)).toContain("Long"));
      await eventually(async () => expect(await atBottom(page!)).toBe(true));
      await page.locator("#content").evaluate((el) => (el.scrollTop = 0));
      await press(page, "k", "ArrowUp");
      await eventually(async () => expect(await activeSection(page!)).toContain("Overview"));

      // gg and G go to the top and the end of the whole document.
      await press(page, "G");
      await eventually(async () => expect(await activeSection(page!)).toContain("Last"));
      await press(page, "g", "g");
      await eventually(async () => expect(await activeSection(page!)).toContain("Overview"));
      await eventually(async () => expect(await contentScroll(page!)).toBe(0));
      await page.locator("#sections .item").nth(2).click(); // Long
      await press(page, "G");
      await eventually(async () => expect(await activeSection(page!)).toContain("Last"));
      await eventually(async () => expect(await atBottom(page!)).toBe(true));
      await press(page, "g", "g");
      // Nothing before the first section.
      await press(page, "k");
      await eventually(async () => expect(await activeSection(page!)).toContain("Overview"));
      // A lone g followed by another key is not a chord; Tab does nothing.
      await press(page, "g", "j", "Tab");
      await eventually(async () => expect(await activeSection(page!)).toContain("Code"));
    },
    TEST_TIMEOUT,
  );

  test(
    "page keys scroll by pages and step on at the edges",
    async () => {
      fixture = writeFixture(WIDE_DOC);
      web = await launchWeb({ file: fixture.path });
      page = await openPage(web);
      await page.locator("#sections .item").nth(2).click(); // Long

      await press(page, "Control+d");
      await eventually(async () => expect(await contentScroll(page!)).toBeGreaterThan(100));
      await press(page, "Control+u");
      await eventually(async () => expect(await contentScroll(page!)).toBeLessThanOrEqual(1));
      await press(page, "Control+u"); // at the top: previous section
      await eventually(async () => expect(await activeSection(page!)).toContain("Code"));
      await page.locator("#sections .item").nth(2).click();
      for (let i = 0; i < 12 && !(await activeSection(page)).includes("Last"); i++) await press(page, "Control+f");
      await eventually(async () => expect(await activeSection(page!)).toContain("Last"));
      await press(page, "Control+b");
      await eventually(async () => expect(await activeSection(page!)).toContain("Long"));
    },
    TEST_TIMEOUT,
  );

  test(
    "the full view reads as one page and the section list follows the scroll",
    async () => {
      fixture = writeFixture(WIDE_DOC);
      web = await launchWeb({ file: fixture.path });
      page = await openPage(web);

      await press(page, "f");
      await eventually(async () => expect(await count(page!, "#content .part")).toBe(4)); // one page, no section blocks
      expect(await count(page, "#content .block-head")).toBe(0);
      await eventually(async () => expect(await text(page!, "#statusbar")).toContain("f section"));

      // f scrolled the first section to the top (past the pane padding).
      const start = await contentScroll(page);
      await press(page, "j");
      await eventually(async () => expect(await contentScroll(page!)).toBe(start + 20));
      await press(page, "k");
      await eventually(async () => expect(await contentScroll(page!)).toBe(start));

      // G scrolls to the end; the list follows the section at the top of the
      // pane (syncCursorToScroll), "Long" since "Last" is too short to reach it.
      await press(page, "G");
      await eventually(async () => expect(await activeSection(page!)).toContain("Long"));
      // The full view scrolls on; there is no section to step to.
      await press(page, "j");
      await eventually(async () => expect(await atBottom(page!)).toBe(true));
      await press(page, "g", "g");
      await eventually(async () => expect(await activeSection(page!)).toContain("Overview"));

      // Choosing a section from the list scrolls to it.
      await page.locator("#sections .item").nth(2).click();
      await eventually(async () => expect(await contentScroll(page!)).toBeGreaterThan(100));
      expect(await activeSection(page)).toContain("Long");
    },
    TEST_TIMEOUT,
  );

  test(
    "wheel scrolling runs on across sections, carrying its movement",
    async () => {
      fixture = writeFixture(WIDE_DOC);
      web = await launchWeb({ file: fixture.path });
      page = await openPage(web);
      await page.locator("#sections .item").nth(1).click(); // Code: fits, so already at its end
      await page.locator("#content").hover();

      // Scrolling on moves into Long and keeps the movement there.
      await page.mouse.wheel(0, 400);
      await eventually(async () => expect(await activeSection(page!)).toContain("Long"));
      await eventually(async () => expect(await contentScroll(page!)).toBeGreaterThanOrEqual(300));
      // Continuous scrolling (like momentum) runs through Long into Last.
      for (let i = 0; i < 40 && !(await activeSection(page)).includes("Last"); i++) await page.mouse.wheel(0, 400);
      await eventually(async () => expect(await activeSection(page!)).toContain("Last"));
      // Scrolling up enters Long from its end, still moving.
      await page.mouse.wheel(0, -400);
      await eventually(async () => expect(await activeSection(page!)).toContain("Long"));
      await eventually(async () => expect(await atBottom(page!)).toBe(false));
      expect(await contentScroll(page)).toBeGreaterThan(1000);

      // Not in the full view, which scrolls through everything.
      await press(page, "f", "G");
      await page.mouse.wheel(0, 100);
      await consistently(async () => expect(await count(page!, "#content .part")).toBe(4));
    },
    TEST_TIMEOUT,
  );

  test(
    "raw view: scrolling brings the line cursor along, so keys continue on screen",
    async () => {
      web = await launchWeb({ file: FIXTURE_BASIC });
      page = await openPage(web, { width: 1000, height: 400 });
      await press(page, "r", "f"); // raw full view, cursor on line 1
      await eventually(async () => expect(await cursorLine(page!)).toBe("1"));
      await page.locator("#content").hover();
      await page.mouse.wheel(0, 300);
      // Line 1 went off the top: the cursor moves to the first visible line.
      await eventually(async () => expect(Number(await cursorLine(page!))).toBeGreaterThan(10));
      const line = Number(await cursorLine(page));
      const scroll = await contentScroll(page);
      await press(page, "j");
      await eventually(async () => expect(Number(await cursorLine(page!))).toBe(line + 1));
      expect(Math.abs((await contentScroll(page)) - scroll)).toBeLessThan(25); // no jump back
      // Scrolling back up moves it to the last visible line, above where it was.
      await page.mouse.wheel(0, -300);
      await eventually(async () => expect(Number(await cursorLine(page!))).toBeLessThan(line + 1));
    },
    TEST_TIMEOUT,
  );

  test(
    "raw section view: k moves up through a long section under its sticky header",
    async () => {
      fixture = writeFixture(["## Long", "", ...Array.from({ length: 80 }, (_, i) => `line ${i + 1}`), "", "## Other", "", "x", ""].join("\n"));
      web = await launchWeb({ file: fixture.path });
      page = await openPage(web, { width: 1000, height: 500 });
      await press(page, "r", "G", "k", "k", "k"); // back into the end of Long
      await eventually(async () => expect(await activeSection(page!)).toContain("Long"));
      const start = Number(await cursorLine(page));
      for (let i = 0; i < 40; i++) await press(page, "k");
      await eventually(async () => expect(Number(await cursorLine(page!))).toBe(start - 40));
      // The cursor row is below the header, not hidden under it.
      const [rowTop, headBottom] = await page.evaluate(() => [
        document.querySelector("#content tr.cursor")!.getBoundingClientRect().top,
        document.querySelector("#content .block-head")!.getBoundingClientRect().bottom,
      ]);
      expect(rowTop).toBeGreaterThanOrEqual(headBottom - 1);
    },
    TEST_TIMEOUT,
  );

  test(
    "raw view scrolling keeps the section list where it was",
    async () => {
      web = await launchWeb({ file: "README.md" });
      page = await openPage(web, { width: 1000, height: 400 });
      await press(page, "r", "f");
      await page.locator("#sections .item").last().click(); // scrolls the list down to it
      const listTop = await page.locator("#sections").evaluate((el) => el.scrollTop);
      expect(listTop).toBeGreaterThan(0);
      await page.locator("#content").hover();
      await page.mouse.wheel(0, -60);
      await consistently(async () =>
        expect(Math.abs((await page!.locator("#sections").evaluate((el) => el.scrollTop)) - listTop)).toBeLessThan(60),
      );
    },
    TEST_TIMEOUT,
  );

  test(
    "gg and G reach the ends of the document even when collapsed or filtered",
    async () => {
      fixture = writeFixture("## A\n\na\n\n### A1\n\nchild end\n\n## B\n\nb\n\n### B1\n\nlast\n");
      web = await launchWeb({ file: fixture.path });
      page = await openPage(web);

      // The last section is inside a collapsed parent: G expands it.
      await page.locator("#sections .item").nth(2).locator(".twisty").click(); // collapse B
      await eventually(async () => expect(await count(page!, "#sections .item")).toBe(3));
      await press(page, "G");
      await eventually(async () => expect(await activeSection(page!)).toContain("B1"));
      expect(await count(page, "#sections .item")).toBe(4);
      expect(await text(page, "#content")).toContain("last");

      // A search filter that hides the first section is cleared by gg.
      await press(page, "/");
      await page.keyboard.type("last");
      await press(page, "Enter");
      await eventually(async () => expect(await count(page!, "#sections .item")).toBe(2)); // B1 and its parent
      await press(page, "g", "g");
      await eventually(async () => expect(await activeSection(page!)).toContain("A"));
      expect(await count(page, "#sections .item")).toBe(4);
    },
    TEST_TIMEOUT,
  );

  test(
    "h/l/H/L scroll wide blocks of the rendered view, not the raw view",
    async () => {
      fixture = writeFixture(WIDE_DOC);
      web = await launchWeb({ file: fixture.path });
      page = await openPage(web);

      await press(page, "j"); // Code section
      await eventually(async () => expect(await count(page!, "#content pre")).toBe(1));
      await press(page, "l", "l", "ArrowRight");
      await eventually(async () => expect(await preScroll(page!)).toBe(96));
      await press(page, "h", "ArrowLeft");
      await eventually(async () => expect(await preScroll(page!)).toBe(32));
      await press(page, "L");
      await eventually(async () => expect(await preScroll(page!)).toBeGreaterThan(1000));
      await press(page, "H");
      await eventually(async () => expect(await preScroll(page!)).toBe(0));

      await press(page, "r", "l");
      await eventually(async () => expect(await count(page!, "#content pre")).toBe(0));
    },
    TEST_TIMEOUT,
  );

  test(
    "raw view: line cursor keys, section edges and the full view",
    async () => {
      web = await launchWeb({ file: FIXTURE_BASIC });
      page = await openPage(web);

      await press(page, "r"); // overview lines 1-4
      await eventually(async () => expect(await cursorLine(page!)).toBe("1"));
      expect(await text(page, "#statusbar .indicator")).toBe("L1/31");
      // G and gg go to the last and first line of the document.
      await press(page, "G");
      await eventually(async () => expect(await cursorLine(page!)).toBe("31"));
      expect(await activeSection(page)).toContain("Step 3: Tests");
      await press(page, "g", "g");
      await eventually(async () => expect(await cursorLine(page!)).toBe("1"));
      await press(page, "j", "j", "j");
      await eventually(async () => expect(await cursorLine(page!)).toBe("4"));
      await press(page, "j"); // crosses into S1
      await eventually(async () => expect(await cursorLine(page!)).toBe("5"));
      expect(await activeSection(page)).toContain("Step 1");
      await press(page, "ArrowUp"); // back to the end of the overview
      await eventually(async () => expect(await cursorLine(page!)).toBe("4"));
      expect(await activeSection(page)).toContain("Overview");
      await press(page, "g", "g");
      await eventually(async () => expect(await cursorLine(page!)).toBe("1"));
      // A page key stops at the section end, the next one steps on.
      await press(page, "Control+d");
      await eventually(async () => expect(await cursorLine(page!)).toBe("4"));
      await press(page, "Control+d");
      await eventually(async () => expect(await cursorLine(page!)).toBe("5"));
      await press(page, "Control+u");
      await eventually(async () => expect(await cursorLine(page!)).toBe("4"));

      await press(page, "f"); // full view: all lines
      await eventually(async () => expect(await count(page!, "#content tr[data-idx]")).toBe(31));
      expect(await count(page, "#content .block-head")).toBe(0);
      await press(page, "g", "g", "Control+d");
      await eventually(async () => expect(Number(await cursorLine(page!))).toBeGreaterThan(10));
      await press(page, "Control+u");
      await eventually(async () => expect(await cursorLine(page!)).toBe("1"));
      await press(page, "Control+f");
      await eventually(async () => expect(await cursorLine(page!)).toBe("31"));
      expect(await activeSection(page)).toContain("Step 3: Tests");
      await press(page, "Control+b");
      await eventually(async () => expect(await cursorLine(page!)).toBe("1"));
      await press(page, "ArrowDown", "ArrowDown", "k");
      await eventually(async () => expect(await cursorLine(page!)).toBe("2"));

      // Choosing a section from the list moves the cursor to it.
      await page.locator("#sections .item").last().click();
      await eventually(async () => expect(await cursorLine(page!)).toBe("29"));

      // Back to the section view: the cursor stays on a visible line.
      await press(page, "f");
      await eventually(async () => expect(await count(page!, "#content tr[data-idx]")).toBe(3));
      await press(page, "r");
      await eventually(async () => expect(await count(page!, "#content tr[data-idx]")).toBe(0));
    },
    TEST_TIMEOUT,
  );

  test(
    "> and < resize the left pane within 10% to 50%",
    async () => {
      web = await launchWeb({ file: FIXTURE_BASIC });
      page = await openPage(web);
      const width = () => page!.locator("#sidebar").evaluate((el) => (el as HTMLElement).style.width);

      expect(await width()).toBe("20%"); // list : content = 2 : 8
      await press(page, ">");
      await eventually(async () => expect(await width()).toBe("25%"));
      for (let i = 0; i < 6; i++) await press(page, "<");
      await eventually(async () => expect(await width()).toBe("10%"));
      for (let i = 0; i < 9; i++) await press(page, ">");
      await eventually(async () => expect(await width()).toBe("50%"));
    },
    TEST_TIMEOUT,
  );

  test(
    "help opens with ? and closes with Esc, Enter, ? or q",
    async () => {
      web = await launchWeb({ file: FIXTURE_BASIC });
      page = await openPage(web);
      for (const close of ["Escape", "Enter", "?", "q"]) {
        await press(page, "?");
        await eventually(async () => expect(await text(page!, ".modal.help")).toContain("commd - Help"));
        await press(page, close);
        await eventually(async () => expect(await count(page!, ".modal")).toBe(0));
      }
      // Other keys leave the help open; clicking the backdrop closes it.
      await press(page, "?", "j");
      await eventually(async () => expect(await count(page!, ".modal.help")).toBe(1));
      await page.mouse.click(5, 400);
      await eventually(async () => expect(await count(page!, ".modal")).toBe(0));
      // Nothing was left behind: quitting outputs nothing.
      await press(page, "q", "y");
      expect((await finished(web)).stdout).toBe("");
    },
    TEST_TIMEOUT,
  );
});
