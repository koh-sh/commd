import { describe, test, expect, afterEach } from "bun:test";
import { launchCommd, TEST_TIMEOUT } from "../helpers/session";
import { createRepo } from "../helpers/git-repo";
import type { Session } from "tuistory";

describe("Diff Mode", () => {
  let session: Session;
  let repo: { dir: string; cleanup: () => void } | undefined;

  afterEach(async () => {
    session?.close();
    // Wait for the commd process to exit after SIGTERM before removing the
    // repo it is still running in (same reason as createTempFixture cleanup).
    await Bun.sleep(300);
    repo?.cleanup();
    repo = undefined;
  });

  test("--diff opens raw diff view with added and removed lines", async () => {
    repo = createRepo(true);
    session = await launchCommd({ file: "doc.md", args: ["--diff"], cwd: repo.dir });
    await session.press("f"); // full view: show every diff line
    const text = await session.waitForText("Added by diff test");
    expect(text).toContain("- # Diff Doc");
    expect(text).toContain("+ # Diff Doc v2");
    expect(text).toContain("+ Added by diff test");
    // Diff mode starts in raw view; the status bar offers the rendered view.
    expect(text).toContain("r render");
  }, TEST_TIMEOUT);

  test("comments on removed and added lines are quoted in the output", async () => {
    repo = createRepo(true);
    session = await launchCommd({
      file: "doc.md",
      args: ["--diff", "--output", "stdout"],
      cwd: repo.dir,
    });
    await session.press("f");
    await session.waitForText("Added by diff test");
    // Focus starts on the section list (same as PR mode); move to the diff pane.
    await session.press("tab");

    // First display line is the removed title (old-file line 1).
    await session.press("g");
    await session.press("g");
    await session.press("c");
    await session.waitForText("save");
    await session.type("why drop the old title");
    await session.press(["ctrl", "s"]);
    await session.waitForText("r render");

    // Last display line is the appended line.
    await session.press("G");
    await session.press("c");
    await session.waitForText("save");
    await session.type("new trailing line");
    await session.press(["ctrl", "s"]);
    await session.waitForText("r render");

    await session.press("s");
    await session.waitForText("Submit review?");
    await session.press("y");
    await session.waitIdle({ timeout: 5000 });

    const text = await session.text({ immediate: true });
    expect(text).toContain("(removed)");
    expect(text).toContain("> # Diff Doc");
    expect(text).toContain("why drop the old title");
    expect(text).toContain("> Added by diff test");
    expect(text).toContain("new trailing line");
  }, TEST_TIMEOUT);

  test("--diff without a file lists changed and untracked Markdown files", async () => {
    repo = createRepo(true);
    session = await launchCommd({
      args: ["--diff"],
      cwd: repo.dir,
      waitFor: "Select Markdown files",
    });
    const text = await session.text();
    expect(text).toContain("doc.md");
    expect(text).toContain("new.md");
  }, TEST_TIMEOUT);

  /** Comment on the first visible diff line of the current file, then finish it. */
  async function commentFirstLineAndFinish(s: Session, body: string): Promise<void> {
    await s.press("f");
    await s.press("tab");
    await s.press("g");
    await s.press("g");
    await s.press("c");
    await s.waitForText("save");
    await s.type(body);
    await s.press(["ctrl", "s"]);
    await s.waitForText("r render");
    await s.press("s");
    await s.waitForText("Finish reviewing this file?");
    await s.press("y");
  }

  test("multi-file flow combines per-file comments into one output", async () => {
    repo = createRepo(true);
    session = await launchCommd({
      args: ["--diff", "--output", "stdout"],
      cwd: repo.dir,
      waitFor: "Select Markdown files",
    });
    await session.press("enter"); // both files selected by default

    // doc.md: multi-file dialogs say "Skip this file?" instead of "Quit review?".
    await session.waitForText("Diff Doc v2");
    await session.press("q");
    await session.waitForText("Skip this file?");
    await session.press("n"); // keep reviewing this file
    await commentFirstLineAndFinish(session, "first file comment");

    // new.md
    await session.waitForText("Fresh file");
    await commentFirstLineAndFinish(session, "second file comment");
    await session.waitIdle({ timeout: 5000 });

    const text = await session.text({ immediate: true });
    expect(text).toContain("## doc.md");
    expect(text).toContain("`L1 (removed)` [question] first file comment");
    expect(text).toContain("## new.md");
    expect(text).toContain("`L1` [question] second file comment");
    expect(text).toContain("> # New Doc");
  }, TEST_TIMEOUT);

  test("untracked file is shown as entirely added", async () => {
    repo = createRepo(true);
    session = await launchCommd({ file: "new.md", args: ["--diff"], cwd: repo.dir });
    await session.press("f");
    const text = await session.waitForText("Fresh file");
    expect(text).toContain("+ # New Doc");
    expect(text).toContain("+ Fresh file");
  }, TEST_TIMEOUT);

  test("unchanged file reports no changes and exits", async () => {
    repo = createRepo(false);
    session = await launchCommd({
      file: "doc.md",
      args: ["--diff"],
      cwd: repo.dir,
      waitFor: "No changes",
    });
    const text = await session.text();
    expect(text).toContain("No changes in doc.md vs HEAD");
  }, TEST_TIMEOUT);
});
