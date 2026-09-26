import { describe, test, expect, afterEach } from "bun:test";
import type { Page } from "playwright";
import { TEST_TIMEOUT, FIXTURE_BASIC } from "../helpers/session";
import { createRepo } from "../helpers/git-repo";
import { launchWeb, finished, stopWeb, type WebSession } from "../helpers/web";
import { useBrowser, openPage, closePage, press, eventually, text, count, activeSection } from "../helpers/browser";

// Basic tier: the critical paths of the `commd review --web` page in a real
// browser, checked against the TUI's behavior. web-ui-*.test.ts cover the
// rest of the key bindings and modes in the full suite.

useBrowser();

describe("Web Review UI (Basic)", () => {
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
    "section comment with label and decoration keys, then submit",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      page = await openPage(web);
      await eventually(async () => expect(await text(page!, "#statusbar")).toContain("[0/7 viewed]"));

      await press(page, "j", "c"); // S1
      await page.keyboard.type("jkq typed, not shortcuts");
      await press(page, "Tab", "Control+d"); // question -> nitpick, none -> non-blocking
      await eventually(async () => expect(await text(page!, ".editor-sep")).toBe("Comment [nitpick (non-blocking)]"));
      await press(page, "Control+s");
      await eventually(async () => expect(await page!.locator("#sections .active .badge").innerText()).toBe("1"));

      await press(page, "s");
      await eventually(async () => expect(await text(page!, ".modal")).toContain("Submit review? (1 comments)"));
      await press(page, "y");

      const { code, stdout } = await finished(web);
      expect(code).toBe(0);
      expect(stdout).toContain("## S1: Step 1: Auth Middleware\n[nitpick (non-blocking)] jkq typed, not shortcuts");
    },
    TEST_TIMEOUT,
  );

  test(
    "raw view: line cursor, visual selection and line comment",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      page = await openPage(web);

      await press(page, "j", "r"); // the overview fits, so j moves on to S1; raw view
      await press(page, "j", "V", "j");
      await eventually(async () => expect(await text(page!, "#statusbar .indicator")).toBe("L6-L7"));
      await press(page, "c");
      await eventually(async () => expect(await text(page!, ".editor-sep")).toBe("Comment [question] (L6-L7)"));
      await page.keyboard.type("range");
      await press(page, "Control+s");

      // At the bottom of the section (its trailing blank line 8), j moves on
      // to the next section.
      await press(page, "j", "j");
      await eventually(async () => expect(await activeSection(page!)).toContain("1.1 JWT Verification"));

      await press(page, "s", "y");
      const { stdout } = await finished(web);
      expect(stdout).toContain("`L6-L7` [question] range");
    },
    TEST_TIMEOUT,
  );

  test(
    "comment list: edit to empty deletes, then approve",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      page = await openPage(web);

      await press(page, "j", "c");
      await page.keyboard.type("first");
      await press(page, "Control+s", "C");
      await eventually(async () => expect(await text(page!, ".comment-list")).toContain("Comments on S1"));
      await press(page, "e");
      await page.locator("#editor-body").fill("");
      // As in the TUI, saving an emptied comment deletes it.
      await press(page, "Control+s");
      await eventually(async () => expect(await count(page!, ".comment-list")).toBe(0));
      await eventually(async () => expect(await count(page!, "#sections .badge")).toBe(0));

      await press(page, "s");
      await eventually(async () => expect(await text(page!, ".modal")).toContain("Submit review? (0 comments)"));
      await press(page, "y");
      const { stdout, stderr } = await finished(web);
      expect(stdout).toBe("");
      expect(stderr).toContain("Approved.");
    },
    TEST_TIMEOUT,
  );

  test(
    "tree toggle, search, viewed and quit",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      page = await openPage(web);

      await press(page, "j", "Enter"); // collapse S1
      await eventually(async () => expect(await count(page!, "#sections .item")).toBe(6));
      await press(page, "Enter");
      await eventually(async () => expect(await count(page!, "#sections .item")).toBe(8));

      await press(page, "/");
      await page.keyboard.type("jwt");
      await eventually(async () => expect(await count(page!, "#sections .item")).toBe(2)); // match + ancestor
      await press(page, "ArrowDown", "Enter");
      await eventually(async () => expect(await activeSection(page!)).toContain("1.1 JWT Verification"));
      await press(page, "v");
      await eventually(async () => expect(await text(page!, "#statusbar")).toContain("[1/7 viewed]"));
      await press(page, "/", "Escape");
      await eventually(async () => expect(await count(page!, "#sections .item")).toBe(8));

      await press(page, "q");
      await eventually(async () => expect(await text(page!, ".modal")).toContain("Quit review?"));
      await press(page, "n");
      await eventually(async () => expect(await count(page!, ".modal")).toBe(0));
      await press(page, "q", "y");
      const { code, stdout } = await finished(web);
      expect(code).toBe(0);
      expect(stdout).toBe("");
    },
    TEST_TIMEOUT,
  );

  test(
    "mouse: drag line numbers to comment; other actions wait for the editor",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);
      page = await openPage(web);

      await page.getByRole("button", { name: "Raw" }).click();
      await page.locator("#sections .item").nth(1).click();
      await page.locator("#content td.num").nth(0).dragTo(page.locator("#content td.num").nth(2));
      await eventually(async () => expect(await text(page!, ".editor-sep")).toBe("Comment [question] (L5-L7)"));

      await page.locator("#sections .item").nth(3).click();
      await eventually(async () => expect(await text(page!, "#toast")).toContain("Save (Ctrl+S) or cancel (Esc)"));
      expect(await activeSection(page)).toContain("Step 1: Auth Middleware");

      await page.locator("#editor-body").fill("by mouse");
      await press(page, "Control+s");
      await page.getByRole("button", { name: "Submit" }).click();
      await page.locator(".modal .btn.primary").click();
      const { stdout } = await finished(web);
      expect(stdout).toContain("`L5-L7` [question] by mouse");
    },
    TEST_TIMEOUT,
  );

  test(
    "--diff: picker, then finish or skip each file in order",
    async () => {
      repo = createRepo(true);
      web = await launchWeb(["--diff"], repo.dir);
      page = await openPage(web);

      await eventually(async () => expect(await text(page!, ".picker")).toContain("Select Markdown files to review"));
      await press(page, "Enter"); // all selected, like the TUI picker

      // Files come in sorted order. Finish doc.md with a comment on the
      // removed title line, then skip new.md with ctrl+c.
      await eventually(async () => expect(await text(page!, ".doc-title")).toContain("(doc.md)"));
      await press(page, "c"); // the diff view opens on the removed title line
      await page.keyboard.type("why rename?");
      await press(page, "Control+s", "s");
      await eventually(async () => expect(await text(page!, ".modal")).toContain("Finish reviewing this file? (1 comments)"));
      await press(page, "y");
      await eventually(async () => expect(await text(page!, ".doc-title")).toContain("(new.md)"));
      await press(page, "Control+c");

      const { stdout } = await finished(web);
      expect(stdout).toContain("on: doc.md");
      expect(stdout).toContain("`L1 (removed)` [question] why rename?\n> # Diff Doc");
      expect(stdout).not.toContain("new.md");
    },
    TEST_TIMEOUT,
  );
});
