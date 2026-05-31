import { describe, test, expect, afterEach } from "bun:test";
import {
  launchCommdPR,
  MOCK_PR_URL,
  TEST_TIMEOUT,
} from "../helpers/session";
import {
  createMockGitHubServer,
  type MockGitHubServer,
} from "../helpers/mock-github";
import { defaultConfig, SECOND_MD } from "../helpers/pr-fixtures";
import type { Session } from "tuistory";

// ──────────────────────────────────────────────────────────
// PR Mode Review TUI
// ──────────────────────────────────────────────────────────
describe("PR Mode Review TUI", () => {
  let session: Session;
  let mock: MockGitHubServer;

  afterEach(() => {
    session?.close();
    mock?.close();
  });

  test("PR mode status bar shows submit and quit", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    const text = await session.waitForText("quit", { timeout: 15000 });
    expect(text).toContain("submit");
    expect(text).toContain("quit");
    expect(text).toContain("switch");
  }, TEST_TIMEOUT);

  test("diff view is default when patch exists", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    const text = await session.waitForText("quit", { timeout: 15000 });
    // In raw/diff mode, status bar shows "r render" to toggle TO render mode
    expect(text).toContain("render");
    // Diff content should show + and - lines
    expect(text).toContain("comprehensive");
  }, TEST_TIMEOUT);

  test("r toggles between render and raw view", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("render", { timeout: 15000 });
    // Switch to render mode
    await session.press("r");
    const rendered = await session.waitForText("raw", { timeout: 5000 });
    // Status bar now shows "r raw" to toggle back
    expect(rendered).toContain("raw");
    // Switch back to diff mode
    await session.press("r");
    const diffView = await session.waitForText("render", { timeout: 5000 });
    expect(diffView).toContain("render");
  }, TEST_TIMEOUT);

  test("section comment in render mode", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Switch to render mode (rawView off), focus stays on left pane
    await session.press("r");
    await session.waitForText("raw", { timeout: 5000 });
    // Navigate to a section and add comment
    await session.press("j"); // Step 1
    await session.press("c");
    await session.waitForText("save");
    await session.type("section level feedback");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("[*]");
    expect(text).toContain("section level feedback");
  }, TEST_TIMEOUT);

  test("line comment in diff view", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Switch focus to right pane (diff view)
    await session.press("tab");
    // Navigate down a few lines in diff
    await session.press("j");
    await session.press("j");
    // Add line comment
    await session.press("c");
    const commentText = await session.waitForText("save");
    // Should show line reference
    expect(commentText).toMatch(/\(L\d+\)/);
    await session.type("line level feedback");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("line level feedback");
  }, TEST_TIMEOUT);

  test("range comment in diff view with visual select", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Switch focus to right pane
    await session.press("tab");
    // Start visual select
    await session.press("V");
    await session.waitForText("VISUAL");
    // Extend the selection down to an added (RIGHT) line. The patch is
    // context / removed(LEFT) / added(RIGHT), so we step past the removed
    // line and land the cursor on an added line, keeping a same-side range.
    await session.press("j");
    await session.press("j");
    // Comment on range
    await session.press("c");
    const commentText = await session.waitForText("save");
    // Should show a same-side range like (L1-L2)
    expect(commentText).toMatch(/\(L\d+-L\d+\)/);
    await session.type("range feedback");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("range feedback");
  }, TEST_TIMEOUT);

  test("visual select spanning removed+added lines collapses to a single side (#32)", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Focus the diff pane and move the cursor onto the removed (LEFT) line.
    await session.press("tab");
    await session.press("j"); // context -> removed line
    // Start a visual selection that extends down across the added (RIGHT)
    // line, so the selection spans both sides.
    await session.press("V");
    await session.waitForText("VISUAL");
    await session.press("j"); // extend onto the added (RIGHT) line
    await session.press("c");
    const commentText = await session.waitForText("save");
    // A GitHub multiline comment cannot mix sides, so the range is restricted
    // to the cursor's side (the added line), yielding a single-line ref here
    // rather than an invalid mixed (L2-L2)/cross-side range.
    expect(commentText).toMatch(/\(L\d+\)/);
    expect(commentText).not.toMatch(/\(L\d+-L\d+\)/);
  }, TEST_TIMEOUT);

  test("s shows PR mode confirm: Finish reviewing this file?", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    await session.press("s");
    const text = await session.waitForText("Finish reviewing");
    expect(text).toContain("Finish reviewing this file?");
  }, TEST_TIMEOUT);

  test("c on left pane in raw view does nothing", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Default: rawView=true, focus=FocusLeft
    // Move to a section and try to comment
    await session.press("j");
    await session.press("c");
    // Should still be in normal mode (no comment editor opened)
    const text = await session.text();
    expect(text).not.toContain("save");
    expect(text).toContain("submit");
  }, TEST_TIMEOUT);

  test("edit existing comment in PR mode", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Add comment in render mode
    await session.press("r");
    await session.waitForText("raw", { timeout: 5000 });
    await session.press("j");
    await session.press("c");
    await session.waitForText("save");
    await session.type("original comment");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    // Open comment list and edit
    await session.press("C");
    await session.waitForText("edit");
    await session.press("e");
    await session.waitForText("save");
    // Clear text: ctrl+e (end of line) then ctrl+u (kill to beginning)
    await session.press(["ctrl", "e"]);
    await session.press(["ctrl", "u"]);
    await session.type("edited comment");
    await session.press(["ctrl", "s"]);
    // After editing, returns to comment list mode; press esc to go back
    await session.waitForText("edit");
    await session.press("escape");
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("edited comment");
  }, TEST_TIMEOUT);

  test("delete comment in PR mode", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Add comment
    await session.press("r");
    await session.waitForText("raw", { timeout: 5000 });
    await session.press("j");
    await session.press("c");
    await session.waitForText("save");
    await session.type("to be deleted");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    let text = await session.text();
    expect(text).toContain("[*]");
    // Open comment list and delete
    await session.press("C");
    await session.waitForText("delete");
    await session.press("d");
    // Comment deleted → back to normal mode
    await session.waitForText("quit");
    text = await session.text();
    expect(text).not.toContain("[*]");
    expect(text).not.toContain("to be deleted");
  }, TEST_TIMEOUT);

  test("f toggles fullView in raw/diff mode", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Default raw view shows "f full" to switch to full view
    // Press f to toggle
    await session.press("f");
    // In full view, status bar shows "f section" to switch back
    const text = await session.text();
    expect(text).toContain("section");
  }, TEST_TIMEOUT);

  test("viewed mark toggle works in PR mode", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Switch to render mode so status bar shows viewed count
    await session.press("r");
    await session.waitForText("raw", { timeout: 5000 });
    // Navigate to a section
    await session.press("j"); // Step 1
    // Toggle viewed mark
    await session.press("v");
    const text = await session.waitForText("[✓]", { timeout: 5000 });
    expect(text).toContain("1/");
    // Toggle again to unmark
    await session.press("v");
    const unmarked = await session.waitForText("0/", { timeout: 5000 });
    expect(unmarked).not.toContain("[✓]");
  }, TEST_TIMEOUT);

  test("search filters sections in PR mode", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Switch to render mode (search is on left pane which needs non-raw for /)
    await session.press("r");
    await session.waitForText("raw", { timeout: 5000 });
    // Open search and type query (same pattern as existing search.test.ts)
    await session.press("/");
    await session.type("Routing");
    const text = await session.text({ trimEnd: true });
    expect(text).toContain("Routing");
    expect(text).not.toContain("Auth Middleware");
    // Esc restores all sections
    await session.press("escape");
    const cleared = await session.waitForText("Auth Middleware", { timeout: 5000 });
    expect(cleared).toContain("Routing");
  }, TEST_TIMEOUT);

  test("no changes in section shows message in diff view", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // BASIC_PATCH only covers lines 1-7 (overview + Step 1 area)
    // Navigate to Step 2 which is beyond the diff range
    await session.press("j"); // Step 1
    await session.press("j"); // 1.1
    await session.press("j"); // 1.2
    await session.press("j"); // Step 2
    const text = await session.text();
    expect(text).toContain("No changes in this section");
  }, TEST_TIMEOUT);

  test("multiple comments on same section", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Switch to render mode for section comments
    await session.press("r");
    await session.waitForText("raw", { timeout: 5000 });
    // Add first comment on Step 1
    await session.press("j");
    await session.press("c");
    await session.waitForText("save");
    await session.type("first comment");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    // Add second comment on same section
    await session.press("c");
    await session.waitForText("save");
    await session.type("second comment");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    // Open comment list to verify both exist
    await session.press("C");
    const text = await session.waitForText("delete");
    expect(text).toContain("#1");
    expect(text).toContain("#2");
  }, TEST_TIMEOUT);

  test("raw source view toggle on file without patch", async () => {
    mock = createMockGitHubServer({
      ...defaultConfig(),
      files: [
        { filename: "docs/guide.md", status: "added", content: SECOND_MD },
      ],
    });
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/guide.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Added file without patch starts in render mode
    // Status bar should show "r raw" (toggle TO raw)
    let text = await session.text();
    expect(text).toContain("raw");
    // Press r to switch to raw source view
    await session.press("r");
    text = await session.waitForText("render", { timeout: 5000 });
    // Now in raw view, should show source lines
    expect(text).toContain("render");
    expect(text).toContain("# API Guide");
  }, TEST_TIMEOUT);

  test("q shows PR mode confirm: Skip this file?", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    await session.press("q");
    const text = await session.waitForText("Skip this file?");
    expect(text).toContain("Skip this file?");
  }, TEST_TIMEOUT);
});
