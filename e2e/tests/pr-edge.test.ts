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
import {
  defaultConfig,
  BASIC_MD,
  BASIC_PATCH,
  SECOND_MD,
} from "../helpers/pr-fixtures";
import type { Session } from "tuistory";

// ──────────────────────────────────────────────────────────
// PR Edge Cases
// ──────────────────────────────────────────────────────────
describe("PR Edge Cases", () => {
  let session: Session;
  let mock: MockGitHubServer;

  afterEach(() => {
    session?.close();
    mock?.close();
  });

  test("no MD files shows message", async () => {
    mock = createMockGitHubServer({
      ...defaultConfig(),
      files: [
        { filename: "main.go", status: "modified" },
      ],
    });
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
    });
    await session.waitIdle({ timeout: 10000 });
    const text = await session.text({ immediate: true });
    expect(text).toContain("No Markdown files");
  }, TEST_TIMEOUT);

  test("control characters in PR file names and content are shown escaped", async () => {
    // A PR author controls file names and content; an OSC sequence must be
    // displayed as text, not interpreted by the terminal.
    mock = createMockGitHubServer({
      ...defaultConfig(),
      files: [
        { filename: "docs/evil\x1b]0;PWNED\x07.md", status: "added", content: SECOND_MD },
        { filename: "docs/README.md", status: "added", content: "# Title\x1b]0;PWNED\x07\n\nBody text.\n" },
      ],
    });
    session = await launchCommdPR({ prURL: MOCK_PR_URL, mockServerURL: mock.url });
    let text = await session.waitForText("Select Markdown files", { timeout: 15000 });
    expect(text).toContain("docs/evil\\x1b]0;PWNED\\a.md");
    session.close();

    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    text = await session.waitForText("quit", { timeout: 15000 });
    expect(text).toContain("Title\\x1b]0;PWNED\\a");
  }, TEST_TIMEOUT);

  test("all files skipped exits without ReviewDialog", async () => {
    mock = createMockGitHubServer({
      ...defaultConfig(),
      files: [
        { filename: "docs/README.md", status: "modified", patch: BASIC_PATCH, content: BASIC_MD },
        { filename: "docs/guide.md", status: "added", content: SECOND_MD },
      ],
    });
    session = await launchCommdPR({ prURL: MOCK_PR_URL, mockServerURL: mock.url });
    // File picker
    await session.waitForText("Select Markdown files", { timeout: 15000 });
    await session.press("enter");
    // Skip first file
    await session.waitForText("quit", { timeout: 15000 });
    await session.press("q");
    await session.waitForText("Skip this file?");
    await session.press("y");
    // Skip second file
    await session.waitForText("quit", { timeout: 15000 });
    await session.press("q");
    await session.waitForText("Skip this file?");
    await session.press("y");
    // Process should exit without ReviewDialog
    await session.waitIdle({ timeout: 5000 });
    expect(mock.submittedReviews).toHaveLength(0);
  }, TEST_TIMEOUT);

  test("multi-file mixed flow: comment on file 1, skip file 2", async () => {
    mock = createMockGitHubServer({
      ...defaultConfig(),
      files: [
        { filename: "docs/README.md", status: "modified", patch: BASIC_PATCH, content: BASIC_MD },
        { filename: "docs/guide.md", status: "added", content: SECOND_MD },
      ],
    });
    session = await launchCommdPR({ prURL: MOCK_PR_URL, mockServerURL: mock.url });
    // File picker: confirm all
    await session.waitForText("Select Markdown files", { timeout: 15000 });
    await session.press("enter");
    // File 1: add comment and submit
    await session.waitForText("quit", { timeout: 15000 });
    await session.press("r"); // render mode
    await session.waitForText("raw", { timeout: 5000 });
    await session.press("j");
    await session.press("c");
    await session.waitForText("save");
    await session.type("file 1 comment");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    await session.press("s");
    await session.waitForText("Finish reviewing");
    await session.press("y");
    // File 2: skip
    await session.waitForText("quit", { timeout: 15000 });
    await session.press("q");
    await session.waitForText("Skip this file?");
    await session.press("y");
    // ReviewDialog: only file 1 should appear in summary
    const dialogText = await session.waitForText("cancel", { timeout: 15000 });
    expect(dialogText).toContain("docs/README.md");
    expect(dialogText).not.toContain("docs/guide.md");
    // Submit as Comment
    await session.press("enter");
    await session.waitForText("back", { timeout: 5000 });
    await session.press(["ctrl", "s"]);
    await session.waitIdle({ timeout: 10000 });
    // Verify only file 1 comments submitted
    expect(mock.submittedReviews).toHaveLength(1);
    const review = mock.submittedReviews[0];
    expect(review.comments.length).toBeGreaterThanOrEqual(1);
    expect(review.comments[0].path).toBe("docs/README.md");
  }, TEST_TIMEOUT);

  test("overview comment is filtered with warning on submit", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Switch to render mode
    await session.press("r");
    await session.waitForText("raw", { timeout: 5000 });
    // Overview is the first item (cursor starts there), add comment on it
    await session.press("c");
    await session.waitForText("save");
    await session.type("overview comment");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    // Submit
    await session.press("s");
    await session.waitForText("Finish reviewing");
    await session.press("y");
    // ReviewDialog appears — overview comment only, so dialog shows "Comment" option
    await session.waitForText("cancel", { timeout: 15000 });
    await session.press("enter"); // Comment
    await session.waitForText("back", { timeout: 5000 });
    await session.press(["ctrl", "s"]);
    await session.waitIdle({ timeout: 10000 });
    // Overview comments are filtered out during submit.
    // With COMMENT event, empty comments and empty body → "No comments to submit"
    // So no review should be submitted to the mock.
    expect(mock.submittedReviews).toHaveLength(0);
  }, TEST_TIMEOUT);

  test("API submit error prints review content as fallback", async () => {
    mock = createMockGitHubServer({
      ...defaultConfig(),
      reviewStatusCode: 422,
    });
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
    await session.type("important feedback");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    // Submit
    await session.press("s");
    await session.waitForText("Finish reviewing");
    await session.press("y");
    // ReviewDialog
    await session.waitForText("cancel", { timeout: 15000 });
    await session.press("enter"); // Comment
    await session.waitForText("back", { timeout: 5000 });
    await session.press(["ctrl", "s"]);
    await session.waitIdle({ timeout: 10000 });
    // API returned 422 → fallback prints review content to stderr
    const text = await session.text({ immediate: true });
    expect(text).toContain("Review content");
    expect(text).toContain("important feedback");
  }, TEST_TIMEOUT);

  test("GetHeadSHA API error shows error message", async () => {
    mock = createMockGitHubServer({
      ...defaultConfig(),
      pullStatusCode: 404,
    });
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitIdle({ timeout: 10000 });
    const text = await session.text({ immediate: true });
    expect(text).toContain("404");
  }, TEST_TIMEOUT);

  test("ListMDFiles API error shows error message", async () => {
    mock = createMockGitHubServer({
      ...defaultConfig(),
      filesStatusCode: 500,
    });
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
    });
    await session.waitIdle({ timeout: 10000 });
    const text = await session.text({ immediate: true });
    expect(text).toContain("500");
  }, TEST_TIMEOUT);

  test("skip first file proceeds to second file", async () => {
    mock = createMockGitHubServer({
      ...defaultConfig(),
      files: [
        { filename: "docs/README.md", status: "modified", patch: BASIC_PATCH, content: BASIC_MD },
        { filename: "docs/guide.md", status: "added", content: SECOND_MD },
      ],
    });
    session = await launchCommdPR({ prURL: MOCK_PR_URL, mockServerURL: mock.url });
    // File picker
    await session.waitForText("Select Markdown files", { timeout: 15000 });
    await session.press("enter"); // confirm all files
    // First file review TUI
    await session.waitForText("quit", { timeout: 15000 });
    // Skip first file
    await session.press("q");
    await session.waitForText("Skip this file?");
    await session.press("y");
    // Second file review TUI should appear
    const text = await session.waitForText("quit", { timeout: 15000 });
    expect(text).toContain("submit");
  }, TEST_TIMEOUT);
});
