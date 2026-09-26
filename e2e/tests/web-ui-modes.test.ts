import { describe, test, expect, afterEach } from "bun:test";
import type { Page } from "playwright";
import { TEST_TIMEOUT, FIXTURE_BASIC } from "../helpers/session";
import { createRepo } from "../helpers/git-repo";
import { launchWeb, finished, stopWeb, type WebSession } from "../helpers/web";
import { useBrowser, openPage, closePage, press, eventually, consistently, text, count, activeSection, cursorLine } from "../helpers/browser";

// Full suite: the modes of the web page (comment editor, comment list,
// confirm dialog, search, visual selection, file picker) and their keys.

useBrowser();

const stateOf = async (web: WebSession) => (await web.api("GET", "/api/state")).json();

describe("Web Review UI Modes (Full)", () => {
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
    "comment editor: label and decoration cycling, chips, cancel and Ctrl+Enter",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      page = await openPage(web);

      // Esc cancels without a comment; an empty save adds nothing.
      await press(page, "c");
      await page.keyboard.type("dropped");
      await press(page, "Escape");
      await eventually(async () => expect(await count(page!, "#editor")).toBe(0));
      await press(page, "c", "Control+s");
      await eventually(async () => expect(await count(page!, "#editor")).toBe(0));
      expect((await stateOf(web)).file.comments).toEqual([]);

      await press(page, "c", "Shift+Tab"); // question -> issue
      await eventually(async () => expect(await text(page!, ".editor-sep")).toBe("Comment [issue]"));
      for (const want of ["non-blocking", "blocking", "if-minor"]) {
        await press(page, "Control+d");
        await eventually(async () => expect(await text(page!, ".editor-sep")).toBe(`Comment [issue (${want})]`));
      }
      await press(page, "Control+d"); // back to no decoration
      await eventually(async () => expect(await text(page!, ".editor-sep")).toBe("Comment [issue]"));
      await page.locator(".chip.label-praise").click();
      await page.locator(".chip.label-blocking").click();
      await eventually(async () => expect(await text(page!, ".editor-sep")).toBe("Comment [praise (blocking)]"));
      // The textarea keeps the focus, so typing continues.
      await page.keyboard.type("nice");
      await press(page, "Control+Enter");
      await eventually(async () => expect((await stateOf(web!)).file.comments[0]).toMatchObject({
        sectionId: "overview", action: "praise", decoration: "blocking", body: "nice",
      }));
    },
    TEST_TIMEOUT,
  );

  test(
    "comment list: navigate, edit, delete by key and by button",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      page = await openPage(web);

      for (const body of ["one", "two", "three"]) {
        await press(page, "c");
        await page.keyboard.type(body);
        await press(page, "Control+s");
      }
      await eventually(async () => expect((await stateOf(web!)).file.comments.length).toBe(3));

      await press(page, "C");
      await eventually(async () => expect(await text(page!, ".list-item.active")).toContain("#1"));
      await press(page, "j", "j", "j"); // stops at the last
      await eventually(async () => expect(await text(page!, ".list-item.active")).toContain("three"));
      await press(page, "k");
      await eventually(async () => expect(await text(page!, ".list-item.active")).toContain("two"));

      // e edits the selected comment and returns to the list.
      await press(page, "e");
      await eventually(async () => expect(await page!.locator("#editor-body").inputValue()).toBe("two"));
      await page.keyboard.type(" edited");
      await press(page, "Control+s");
      await eventually(async () => expect(await text(page!, ".list-item.active")).toContain("two edited"));

      await press(page, "d"); // delete "two edited"; the list stays open
      await eventually(async () => expect(await count(page!, ".list-item")).toBe(2));
      await page.locator(".list-item").nth(1).locator("button", { hasText: "Delete" }).click();
      await eventually(async () => expect(await count(page!, ".list-item")).toBe(1));
      await page.locator(".list-item button", { hasText: "Edit" }).click();
      await eventually(async () => expect(await page!.locator("#editor-body").inputValue()).toBe("one"));
      await press(page, "Escape"); // back to the list
      await eventually(async () => expect(await count(page!, ".comment-list")).toBe(1));
      await page.locator(".list-item").first().click();
      await press(page, "x", "Escape"); // other keys are ignored; Esc closes
      await eventually(async () => expect(await count(page!, ".comment-list")).toBe(0));
      expect((await stateOf(web)).file.comments.map((c: { body: string }) => c.body)).toEqual(["one"]);

      // C with no comments does nothing.
      await press(page, "j", "C");
      await eventually(async () => expect(await count(page!, ".comment-list")).toBe(0));
    },
    TEST_TIMEOUT,
  );

  test(
    "confirm dialog: n, N, q and Esc cancel; buttons work",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      page = await openPage(web);
      for (const cancel of ["n", "N", "q", "Escape"]) {
        await press(page, "s");
        await eventually(async () => expect(await text(page!, ".modal")).toContain("Submit review? (0 comments)"));
        await press(page, cancel);
        await eventually(async () => expect(await count(page!, ".modal")).toBe(0));
      }
      await page.getByRole("button", { name: "Quit" }).click();
      await eventually(async () => expect(await text(page!, ".modal")).toContain("Quit review?"));
      await page.locator(".modal button", { hasText: "no" }).click();
      await eventually(async () => expect(await count(page!, ".modal")).toBe(0));

      await press(page, "c");
      await page.keyboard.type("keep");
      await press(page, "Control+s", "q");
      await eventually(async () => expect(await text(page!, ".modal")).toContain("You have review comments.\n\nQuit without submitting?"));
      await press(page, "Y");
      const { stdout } = await finished(web);
      expect(stdout).toBe("");
    },
    TEST_TIMEOUT,
  );

  test(
    "search: arrow keys, no match, Tab stays in the input, click to reopen",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      page = await openPage(web);

      await press(page, "/");
      await page.keyboard.type("zzz");
      await eventually(async () => expect(await text(page!, "#sections")).toContain("No matches"));
      await press(page, "Backspace", "Backspace", "Backspace");
      await page.keyboard.type("routing");
      // Like the TUI, a matching section shows with its subsections.
      await eventually(async () => expect(await count(page!, "#sections .item")).toBe(3));
      expect(await text(page, "#sections")).toContain("2.2 Validation");
      await press(page, "ArrowDown", "ArrowDown", "ArrowUp");
      await eventually(async () => expect(await activeSection(page!)).toContain("2.1 Endpoint Addition"));
      await press(page, "Tab");
      expect(await page.evaluate(() => document.activeElement?.id)).toBe("search");
      await page.locator("#search").fill("");
      await page.keyboard.type("step 3");
      await eventually(async () => expect(await count(page!, "#sections .item")).toBe(1));
      await press(page, "Enter");
      await eventually(async () => expect(await activeSection(page!)).toContain("Step 3"));

      // The kept filter shows a read-only input; clicking it starts a new search.
      await page.locator("#search").click();
      await eventually(async () => expect(await page!.locator("#search").inputValue()).toBe(""));
      await eventually(async () => expect(await count(page!, "#sections .item")).toBe(8));
      await page.keyboard.type("overview");
      await eventually(async () => expect(await count(page!, "#sections .item")).toBe(1));
      // Clicking a section ends the search, keeping the filter.
      await page.locator("#sections .item").first().click();
      await eventually(async () => expect(await page!.locator("#search").getAttribute("readonly")).toBe(""));

      // With no pane focus, / starts a search from anywhere.
      await press(page, "r", "/");
      await eventually(async () => expect(await page!.locator("#search").getAttribute("readonly")).toBeNull());
      await press(page, "Escape");
    },
    TEST_TIMEOUT,
  );

  test(
    "visual selection: Esc cancels, the range keeps to the cursor's diff side, C at the cursor",
    async () => {
      repo = createRepo(true);
      web = await launchWeb(["--diff", "doc.md"], repo.dir);
      page = await openPage(web);

      await eventually(async () => expect(await cursorLine(page!)).toBe("1"));
      await press(page, "V", "j");
      await eventually(async () => expect(await count(page!, "#content tr.sel")).toBe(1)); // the + line only
      await press(page, "Escape");
      await eventually(async () => expect(await count(page!, "#content tr.sel")).toBe(0));
      expect(await text(page, "#statusbar")).not.toContain("VISUAL");

      // Removed line 1 (old file) then added line 1 (new file): a comment
      // covers one side, the cursor's.
      await press(page, "k", "V", "j", "c");
      await eventually(async () => expect(await text(page!, ".editor-sep")).toBe("Comment [question] (L1)"));
      await page.keyboard.type("new title");
      await press(page, "Control+s");
      await eventually(async () => expect((await stateOf(web!)).file.comments[0]).toMatchObject({ startLine: 1, side: "RIGHT" }));

      // C lists the comments of the section under the cursor.
      await press(page, "C");
      await eventually(async () => expect(await text(page!, ".comment-list")).toContain("new title"));
      await press(page, "Escape", "k", "c");
      await eventually(async () => expect(await text(page!, ".editor-sep")).toBe("Comment [question] (L1)"));
      await page.keyboard.type("old title");
      await press(page, "Control+s", "s", "y");
      const { stdout } = await finished(web);
      expect(stdout).toContain("`L1` [question] new title\n> # Diff Doc v2");
      expect(stdout).toContain("`L1 (removed)` [question] old title\n> # Diff Doc");
    },
    TEST_TIMEOUT,
  );

  test(
    "picker: keys, clicks and buttons; nothing selected reviews nothing",
    async () => {
      repo = createRepo(true);
      web = await launchWeb(["--diff"], repo.dir);
      page = await openPage(web);

      await eventually(async () => expect(await text(page!, ".picker li.active")).toContain("[✓] doc.md"));
      await press(page, "j", " ");
      await eventually(async () => expect(await text(page!, ".picker li.active")).toContain("[ ] new.md"));
      await press(page, "k", "ArrowDown", "ArrowUp");
      await eventually(async () => expect(await text(page!, ".picker li.active")).toContain("doc.md"));
      await press(page, "a"); // not all selected -> select all
      await eventually(async () => expect(await count(page!, ".picker li", )).toBe(2));
      expect(await text(page, ".picker")).not.toContain("[ ]");
      await press(page, "a"); // all selected -> none
      await eventually(async () => expect(await text(page!, ".picker")).not.toContain("[✓]"));
      await page.locator(".picker li").nth(1).click(); // select new.md only
      await eventually(async () => expect(await text(page!, ".picker li.active")).toContain("[✓] new.md"));
      await page.getByRole("button", { name: "Confirm" }).click();
      await eventually(async () => expect(await text(page!, ".doc-title")).toContain("(new.md)"));
      // A single picked file is not a multi-file review.
      expect(await page.getByRole("button", { name: "Submit" }).count()).toBe(1);
      await press(page, "s", "y");
      const { stderr } = await finished(web);
      expect(stderr).toContain("Approved.");
    },
    TEST_TIMEOUT,
  );

  test(
    "picker: q and the Cancel button cancel",
    async () => {
      for (const cancel of ["q", "button"]) {
        repo = createRepo(true);
        web = await launchWeb(["--diff"], repo.dir);
        page = await openPage(web);
        await eventually(async () => expect(await count(page!, ".picker")).toBe(1));
        if (cancel === "q") await press(page, "q");
        else await page.getByRole("button", { name: "Cancel" }).click();
        const { code, stdout } = await finished(web);
        expect([code, stdout]).toEqual([0, ""]);
        await closePage(page);
        page = undefined;
        repo.cleanup();
        repo = undefined;
      }
    },
    TEST_TIMEOUT,
  );

  test(
    "Ctrl+C copies selected text and otherwise quits (skips in multi-file)",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      page = await openPage(web);

      await page.locator("#content .markdown p").first().selectText();
      await press(page, "Control+c");
      await consistently(() => expect(web!.proc.exitCode).toBeNull()); // still running
      await page.evaluate(() => window.getSelection()?.removeAllRanges());
      await press(page, "Control+c");
      const { code, stdout } = await finished(web);
      expect([code, stdout]).toEqual([0, ""]);
    },
    TEST_TIMEOUT,
  );
});
