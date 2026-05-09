import { describe, test, expect, afterEach } from "bun:test";
import { launchTerminal } from "tuistory";
import {
  launchCommd,
  addComment,
  createTempFixture,
  FIXTURE_BASIC,
  TEST_TIMEOUT,
} from "../helpers/session";
import { resolve } from "path";
import { writeFileSync, readFileSync, unlinkSync } from "fs";
import type { Session } from "tuistory";

const PROJECT_ROOT = resolve(import.meta.dir, "../..");
const COMMD_BIN = resolve(PROJECT_ROOT, "commd");

/** Launch commd review in a narrow terminal (status bar truncates "quit"). */
async function launchNarrow(
  file: string,
  cols: number,
  rows = 36,
): Promise<Session> {
  const session = await launchTerminal({
    command: COMMD_BIN,
    args: ["review", file],
    cols,
    rows,
    cwd: PROJECT_ROOT,
    env: { TERM: "xterm-256color", CI: "true" },
    waitForData: true,
    waitForDataTimeout: 10000,
  });
  // Status bar at narrow width truncates "quit", so wait for "comment" instead
  await session.waitForText("comment", { timeout: 15000 });
  return session;
}

// ──────────────────────────────────────────────────────────
// High Priority
// ──────────────────────────────────────────────────────────

describe("Narrow Terminal (single-pane mode)", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("width < 80 renders single pane", async () => {
    session = await launchNarrow(FIXTURE_BASIC, 60);
    const text = await session.text();
    // Single pane: the double-border "││" pattern should NOT appear
    expect(text).not.toContain("││");
    expect(text).toContain("Overview");
  }, TEST_TIMEOUT);

  test("tab switches visible pane in single-pane mode", async () => {
    session = await launchNarrow(FIXTURE_BASIC, 60);
    // Initially left pane (section list) is shown
    const left = await session.text();
    expect(left).toContain("Overview");
    // Tab to right pane (detail)
    await session.press("tab");
    const right = await session.text();
    // Right pane shows rendered detail content
    expect(right).toContain("Authentication");
  }, TEST_TIMEOUT);
});

describe("--track-viewed Persistence", () => {
  let session: Session;
  let fixture: { path: string; cleanup: () => void };

  afterEach(async () => {
    session?.close();
    await fixture?.cleanup();
  });

  test("viewed marks persist across sessions", async () => {
    fixture = createTempFixture(FIXTURE_BASIC);

    // Session 1: mark Step 1 as viewed, then quit
    session = await launchCommd({
      file: fixture.path,
      args: ["--track-viewed"],
    });
    await session.press("j"); // Step 1
    await session.press("v"); // toggle viewed
    await session.waitForText("[✓]");
    await session.press("q");
    await session.waitForText("Quit review?");
    await session.press("y");
    await session.waitIdle({ timeout: 5000 });
    session.close();

    // Session 2: viewed mark should be restored
    session = await launchCommd({
      file: fixture.path,
      args: ["--track-viewed"],
    });
    const text = await session.text();
    expect(text).toContain("[✓]");
    expect(text).toContain("1/");
  }, TEST_TIMEOUT);
});

describe("Content Hash Invalidation", () => {
  let session: Session;
  let fixture: { path: string; cleanup: () => void };

  afterEach(async () => {
    session?.close();
    await fixture?.cleanup();
  });

  test("viewed mark clears when content changes", async () => {
    fixture = createTempFixture(FIXTURE_BASIC);
    const absPath = resolve(PROJECT_ROOT, fixture.path);

    // Session 1: mark Step 1 as viewed
    session = await launchCommd({
      file: fixture.path,
      args: ["--track-viewed"],
    });
    await session.press("j");
    await session.press("v");
    await session.waitForText("[✓]");
    await session.press("q");
    await session.waitForText("Quit review?");
    await session.press("y");
    await session.waitIdle({ timeout: 5000 });
    session.close();

    // Modify the fixture content (change Step 1 body)
    let content = readFileSync(absPath, "utf-8");
    content = content.replace(
      "Implement authentication middleware",
      "Implement updated authentication middleware",
    );
    writeFileSync(absPath, content);

    // Session 2: viewed mark should be cleared due to content change
    session = await launchCommd({
      file: fixture.path,
      args: ["--track-viewed"],
    });
    const text = await session.text();
    expect(text).not.toContain("[✓]");
    expect(text).toContain("0/");
  }, TEST_TIMEOUT);
});

describe("Overview Comment", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("can add comment on Overview section", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    // Cursor starts on Overview (first item)
    await session.press("c");
    await session.waitForText("save");
    await session.type("overall note");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("overall note");
    expect(text).toContain("[*]");
  }, TEST_TIMEOUT);
});

describe("Output Modes", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("--output stdout prints review to terminal", async () => {
    session = await launchCommd({
      file: FIXTURE_BASIC,
      args: ["--output", "stdout"],
    });
    await session.press("j"); // Step 1
    await addComment(session, "stdout test");
    await session.press("s");
    await session.waitForText("Submit review?");
    await session.press("y");
    await session.waitIdle({ timeout: 5000 });
    const text = await session.text({ immediate: true });
    // Review output printed to stdout (which goes to the terminal)
    expect(text).toContain("stdout test");
  }, TEST_TIMEOUT);

  test("--output file writes review to file", async () => {
    const outPath = resolve(PROJECT_ROOT, "e2e/tests/.tmp-review-output.md");
    // Pre-create the file (required by review.go's os.Stat check)
    writeFileSync(outPath, "");
    session = await launchCommd({
      file: FIXTURE_BASIC,
      args: ["--output", "file", "--output-path", outPath],
    });
    await session.press("j");
    await addComment(session, "file output test");
    await session.press("s");
    await session.waitForText("Submit review?");
    await session.press("y");
    await session.waitIdle({ timeout: 5000 });
    // Verify the output file was written with review content
    const output = readFileSync(outPath, "utf-8");
    expect(output).toContain("file output test");
    // Cleanup
    try { unlinkSync(outPath); } catch {}
  }, TEST_TIMEOUT);
});

// ──────────────────────────────────────────────────────────
// Medium Priority
// ──────────────────────────────────────────────────────────

describe("Comment Editor", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("Tab cycles through all labels and wraps around", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("c");
    await session.waitForText("label: question");
    // Cycle forward: question → nitpick → todo → thought → note → praise → chore → suggestion → issue → question
    const labels = ["nitpick", "todo", "thought", "note", "praise", "chore", "suggestion", "issue", "question"];
    for (const label of labels) {
      await session.press("tab");
      await session.waitForText(`label: ${label}`, { timeout: 3000 });
    }
    // After full cycle, should be back to question
    const text = await session.text();
    expect(text).toContain("label: question");
  }, TEST_TIMEOUT);

  test("saving empty body from edit cancels edit, preserves comment", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await addComment(session, "preserved comment");
    // Open comment list and edit
    await session.press("C");
    await session.waitForText("edit");
    await session.press("e");
    await session.waitForText("save");
    // Clear the text
    await session.press(["ctrl", "e"]);
    await session.press(["ctrl", "u"]);
    // Save empty → Result() returns nil → edit is cancelled
    await session.press(["ctrl", "s"]);
    // Returns to CommentList mode (since editCommentIdx >= 0 → returnFromComment → ModeCommentList)
    await session.waitForText("edit");
    // Comment should still exist unchanged
    const text = await session.text();
    expect(text).toContain("preserved comment");
    expect(text).toContain("#1");
  }, TEST_TIMEOUT);
});

describe("View Mode Combinations", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("full view + raw view toggle combination", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    // Toggle to full view
    await session.press("f");
    let text = await session.text();
    expect(text).toContain("section");
    // Toggle to raw view within full view
    await session.press("r");
    text = await session.waitForText("render", { timeout: 5000 });
    expect(text).toContain("render");
    // Toggle raw off
    await session.press("r");
    await session.waitForText("raw", { timeout: 5000 });
    // Toggle full view off
    await session.press("f");
    text = await session.text();
    expect(text).toContain("full");
  }, TEST_TIMEOUT);
});

describe("Raw View Section Boundary", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("cursor crosses section boundary in raw view", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j"); // Step 1
    await session.press("r"); // raw view
    await session.waitForText("render", { timeout: 5000 });
    // Navigate down past section boundary
    for (let i = 0; i < 10; i++) {
      await session.press("j");
    }
    // Should not crash; cursor should have moved
    const text = await session.text();
    expect(text).toContain("render");
  }, TEST_TIMEOUT);
});

// ──────────────────────────────────────────────────────────
// Low Priority
// ──────────────────────────────────────────────────────────

describe("Ctrl+C Exit", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("Ctrl+C in normal mode exits immediately", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press(["ctrl", "c"]);
    await session.waitIdle({ timeout: 5000 });
    const text = await session.text({ immediate: true });
    expect(text).not.toContain("Overview");
  }, TEST_TIMEOUT);

  test("Ctrl+C in confirm dialog exits", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("q");
    await session.waitForText("Quit review?");
    await session.press(["ctrl", "c"]);
    await session.waitIdle({ timeout: 5000 });
    const text = await session.text({ immediate: true });
    expect(text).not.toContain("Overview");
  }, TEST_TIMEOUT);
});

describe("Search Edge Cases", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("search with no matching results", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("/");
    await session.type("xyznonexistent");
    const text = await session.text({ trimEnd: true });
    // No sections should be visible (all filtered out)
    expect(text).not.toContain("Step 1");
    expect(text).not.toContain("Overview");
  }, TEST_TIMEOUT);

  test("j/k navigates within filtered search results", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("/");
    await session.type("Step");
    // Confirm search
    await session.press("enter");
    await session.waitForText("quit");
    // Navigate within filtered results
    await session.press("j");
    await session.press("j");
    const text = await session.text();
    expect(text).toContain("Step");
  }, TEST_TIMEOUT);
});

describe("Help Close Keys", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("q closes help overlay", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("?");
    await session.waitForText("Help");
    await session.press("q");
    const text = await session.waitForText("quit", { timeout: 5000 });
    expect(text).not.toContain("Help");
    expect(text).toContain("Overview");
  }, TEST_TIMEOUT);
});

describe("Horizontal Scroll", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("H scrolls detail pane to start", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    // Press H (scroll to start) - should not error
    await session.press("H");
    const text = await session.text();
    expect(text).toContain("Overview");
  }, TEST_TIMEOUT);
});

describe("Comment List Boundary", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("j at last comment does not crash", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await addComment(session, "comment 1");
    await addComment(session, "comment 2");
    // Open comment list
    await session.press("C");
    await session.waitForText("delete");
    // Navigate past end
    await session.press("j");
    await session.press("j");
    await session.press("j"); // should be clamped
    const text = await session.text();
    expect(text).toContain("#2");
  }, TEST_TIMEOUT);

  // TODO: After upgrading the charmbracelet stack to v2, tuistory's text()
  // returns the previous frame's `#3` as a leftover cell after the delete
  // re-renders the comment list. The production View() output is correct
  // (verified via debug dump). Whether the bug is in tuistory or in the
  // Bubble Tea v2 cell-diff renderer is not yet investigated. Re-enable
  // after fixing.
  // test("delete middle comment from list of 3", async () => {
  //   session = await launchCommd({ file: FIXTURE_BASIC });
  //   await session.press("j");
  //   await addComment(session, "first");
  //   await addComment(session, "second");
  //   await addComment(session, "third");
  //   // Open comment list
  //   await session.press("C");
  //   await session.waitForText("delete");
  //   // Navigate to #2 (middle) and delete
  //   await session.press("j");
  //   await session.press("d");
  //   const text = await session.text();
  //   // Should have #1 and #2 (renumbered from #1 and #3)
  //   expect(text).toContain("#1");
  //   expect(text).toContain("#2");
  //   expect(text).not.toContain("#3");
  // }, TEST_TIMEOUT);
});

describe("Pane Resize Constraint", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("resize is blocked when terminal is narrow", async () => {
    session = await launchNarrow(FIXTURE_BASIC, 60);
    // In single-pane mode (width < 80), > and < should have no effect
    await session.press(">");
    await session.press(">");
    const text = await session.text();
    // Still in single-pane mode
    expect(text).not.toContain("││");
  }, TEST_TIMEOUT);
});
