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
  STEP1_PATCH,
  SECOND_MD,
} from "../helpers/pr-fixtures";
import type { Session } from "tuistory";

// ──────────────────────────────────────────────────────────
// File Picker
// ──────────────────────────────────────────────────────────
describe("PR File Picker", () => {
  let session: Session;
  let mock: MockGitHubServer;

  afterEach(() => {
    session?.close();
    mock?.close();
  });

  test("shows changed MD files with Select title", async () => {
    mock = createMockGitHubServer({
      ...defaultConfig(),
      files: [
        { filename: "docs/README.md", status: "modified", content: BASIC_MD },
        { filename: "docs/guide.md", status: "added", content: SECOND_MD },
      ],
    });
    session = await launchCommdPR({ prURL: MOCK_PR_URL, mockServerURL: mock.url });
    const text = await session.waitForText("Select Markdown files", { timeout: 15000 });
    expect(text).toContain("docs/README.md");
    expect(text).toContain("docs/guide.md");
  }, TEST_TIMEOUT);

  test("all files selected by default", async () => {
    mock = createMockGitHubServer({
      ...defaultConfig(),
      files: [
        { filename: "docs/README.md", status: "modified", content: BASIC_MD },
        { filename: "docs/guide.md", status: "added", content: SECOND_MD },
      ],
    });
    session = await launchCommdPR({ prURL: MOCK_PR_URL, mockServerURL: mock.url });
    const text = await session.waitForText("Select Markdown files", { timeout: 15000 });
    // Count checkmarks - should match number of files
    const checks = text.match(/\[✓\]/g);
    expect(checks).not.toBeNull();
    expect(checks!.length).toBe(2);
  }, TEST_TIMEOUT);

  test("space toggles file selection", async () => {
    mock = createMockGitHubServer({
      ...defaultConfig(),
      files: [
        { filename: "docs/README.md", status: "modified", content: BASIC_MD },
        { filename: "docs/guide.md", status: "added", content: SECOND_MD },
      ],
    });
    session = await launchCommdPR({ prURL: MOCK_PR_URL, mockServerURL: mock.url });
    await session.waitForText("Select Markdown files", { timeout: 15000 });
    // Deselect first file
    await session.press(" ");
    const text = await session.text();
    expect(text).toContain("[ ]");
  }, TEST_TIMEOUT);

  test("a toggles all files selection", async () => {
    mock = createMockGitHubServer({
      ...defaultConfig(),
      files: [
        { filename: "docs/README.md", status: "modified", content: BASIC_MD },
        { filename: "docs/guide.md", status: "added", content: SECOND_MD },
      ],
    });
    session = await launchCommdPR({ prURL: MOCK_PR_URL, mockServerURL: mock.url });
    await session.waitForText("Select Markdown files", { timeout: 15000 });
    // All selected by default; press a to deselect all
    await session.press("a");
    let text = await session.text();
    expect(text).not.toContain("[✓]");
    // Press a again to select all
    await session.press("a");
    text = await session.text();
    const checks = text.match(/\[✓\]/g);
    expect(checks).not.toBeNull();
    expect(checks!.length).toBe(2);
  }, TEST_TIMEOUT);

  test("all files deselected then enter exits without review", async () => {
    mock = createMockGitHubServer({
      ...defaultConfig(),
      files: [
        { filename: "docs/README.md", status: "modified", content: BASIC_MD },
        { filename: "docs/guide.md", status: "added", content: SECOND_MD },
      ],
    });
    session = await launchCommdPR({ prURL: MOCK_PR_URL, mockServerURL: mock.url });
    await session.waitForText("Select Markdown files", { timeout: 15000 });
    // Deselect all
    await session.press("a");
    await session.press("enter");
    await session.waitIdle({ timeout: 5000 });
    expect(mock.submittedReviews).toHaveLength(0);
  }, TEST_TIMEOUT);

  test("q cancels without review", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({ prURL: MOCK_PR_URL, mockServerURL: mock.url });
    await session.waitForText("Select Markdown files", { timeout: 15000 });
    await session.press("q");
    await session.waitIdle({ timeout: 5000 });
    // Process should have exited; no review submitted
    expect(mock.submittedReviews).toHaveLength(0);
  }, TEST_TIMEOUT);

  test("enter confirms and transitions to review TUI", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({ prURL: MOCK_PR_URL, mockServerURL: mock.url });
    await session.waitForText("Select Markdown files", { timeout: 15000 });
    await session.press("enter");
    // Review TUI should appear with status bar
    const text = await session.waitForText("quit", { timeout: 15000 });
    expect(text).toContain("submit");
  }, TEST_TIMEOUT);
});

// ──────────────────────────────────────────────────────────
// --file flag
// ──────────────────────────────────────────────────────────
describe("PR --file flag", () => {
  let session: Session;
  let mock: MockGitHubServer;

  afterEach(() => {
    session?.close();
    mock?.close();
  });

  test("skips picker and shows review TUI directly", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    const text = await session.waitForText("quit", { timeout: 15000 });
    expect(text).toContain("submit");
    // Should NOT show file picker
    expect(text).not.toContain("Select Markdown files");
  }, TEST_TIMEOUT);

  test("nonexistent file shows error", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "missing.md",
    });
    await session.waitIdle({ timeout: 10000 });
    const text = await session.text({ immediate: true });
    expect(text).toContain("not found");
  }, TEST_TIMEOUT);
});

// ──────────────────────────────────────────────────────────
// Review Dialog
// ──────────────────────────────────────────────────────────
describe("Review Dialog", () => {
  let session: Session;
  let mock: MockGitHubServer;

  afterEach(() => {
    session?.close();
    mock?.close();
  });

  /** Helper: navigate through review TUI to ReviewDialog with a comment */
  async function goToReviewDialogWithComment(s: Session): Promise<void> {
    // Switch to render mode for section comment
    await s.press("r");
    await s.waitForText("raw", { timeout: 5000 });
    // Add comment on first real section
    await s.press("j");
    await s.press("c");
    await s.waitForText("save");
    await s.type("test comment");
    await s.press(["ctrl", "s"]);
    await s.waitForText("quit");
    // Submit
    await s.press("s");
    await s.waitForText("Finish reviewing");
    await s.press("y");
  }

  /** Helper: navigate through review TUI to ReviewDialog without comments */
  async function goToReviewDialogNoComment(s: Session): Promise<void> {
    await s.press("s");
    await s.waitForText("Finish reviewing");
    await s.press("y");
  }

  test("with comments shows Comment, Approve, Exit options", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    await goToReviewDialogWithComment(session);
    const text = await session.waitForText("cancel", { timeout: 15000 });
    expect(text).toContain("Comment");
    expect(text).toContain("Approve");
    expect(text).toContain("Exit");
  }, TEST_TIMEOUT);

  test("without comments shows Approve and Exit only", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    await goToReviewDialogNoComment(session);
    const text = await session.waitForText("cancel", { timeout: 15000 });
    expect(text).toContain("Approve");
    expect(text).toContain("Exit");
    // "Comment" option should not be present when there are no comments
    // The dialog should show "No comments" in summary
    expect(text).toContain("No comments");
  }, TEST_TIMEOUT);

  test("Comment selection submits with event COMMENT", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    await goToReviewDialogWithComment(session);
    await session.waitForText("cancel", { timeout: 15000 });
    // "Comment" is the first option (cursor is already there)
    await session.press("enter");
    // Body input mode
    await session.waitForText("back", { timeout: 5000 });
    await session.press(["ctrl", "s"]);
    await session.waitIdle({ timeout: 10000 });
    expect(mock.submittedReviews).toHaveLength(1);
    expect(mock.submittedReviews[0].event).toBe("COMMENT");
  }, TEST_TIMEOUT);

  test("Approve selection submits with event APPROVE", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    await goToReviewDialogWithComment(session);
    await session.waitForText("cancel", { timeout: 15000 });
    // Move to Approve (second option)
    await session.press("j");
    await session.press("enter");
    await session.waitForText("back", { timeout: 5000 });
    await session.press(["ctrl", "s"]);
    await session.waitIdle({ timeout: 10000 });
    expect(mock.submittedReviews).toHaveLength(1);
    expect(mock.submittedReviews[0].event).toBe("APPROVE");
  }, TEST_TIMEOUT);

  test("body text is included in submitted review", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    await goToReviewDialogWithComment(session);
    await session.waitForText("cancel", { timeout: 15000 });
    // Select Comment
    await session.press("enter");
    await session.waitForText("back", { timeout: 5000 });
    // Type body text
    await session.type("Overall looks good");
    await session.press(["ctrl", "s"]);
    await session.waitIdle({ timeout: 10000 });
    expect(mock.submittedReviews).toHaveLength(1);
    expect(mock.submittedReviews[0].body).toContain("Overall looks good");
  }, TEST_TIMEOUT);

  test("Exit selection cancels without submitting", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    await goToReviewDialogWithComment(session);
    await session.waitForText("cancel", { timeout: 15000 });
    // Move to Exit (third option)
    await session.press("j");
    await session.press("j");
    await session.press("enter");
    await session.waitIdle({ timeout: 5000 });
    expect(mock.submittedReviews).toHaveLength(0);
  }, TEST_TIMEOUT);

  test("q in select mode cancels without submitting", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    await goToReviewDialogWithComment(session);
    await session.waitForText("cancel", { timeout: 15000 });
    // Press q to cancel (different code path from selecting Exit)
    await session.press("q");
    await session.waitIdle({ timeout: 5000 });
    expect(mock.submittedReviews).toHaveLength(0);
  }, TEST_TIMEOUT);

  test("esc in body input returns to action selection", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    await goToReviewDialogWithComment(session);
    await session.waitForText("cancel", { timeout: 15000 });
    // Select Comment → body mode
    await session.press("enter");
    await session.waitForText("back", { timeout: 5000 });
    // Press esc to go back
    await session.press("escape");
    // Should be back in select mode with navigation help
    const text = await session.waitForText("navigate", { timeout: 5000 });
    expect(text).toContain("Comment");
    expect(text).toContain("Approve");
  }, TEST_TIMEOUT);
});

// ──────────────────────────────────────────────────────────
// Submit Verification
// ──────────────────────────────────────────────────────────
describe("Submit Verification", () => {
  let session: Session;
  let mock: MockGitHubServer;

  afterEach(() => {
    session?.close();
    mock?.close();
  });

  test("inline comments are included in submitted review", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Add section comment in render mode
    await session.press("r");
    await session.waitForText("raw", { timeout: 5000 });
    await session.press("j"); // Step 1
    await session.press("c");
    await session.waitForText("save");
    await session.type("needs refactoring");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    // Submit
    await session.press("s");
    await session.waitForText("Finish reviewing");
    await session.press("y");
    // ReviewDialog: select Comment → submit
    await session.waitForText("cancel", { timeout: 15000 });
    await session.press("enter");
    await session.waitForText("back", { timeout: 5000 });
    await session.press(["ctrl", "s"]);
    await session.waitIdle({ timeout: 10000 });
    // Verify submitted review
    expect(mock.submittedReviews).toHaveLength(1);
    const review = mock.submittedReviews[0];
    expect(review.event).toBe("COMMENT");
    expect(review.comments.length).toBeGreaterThanOrEqual(1);
    expect(review.comments[0].path).toBe("docs/README.md");
    expect(review.comments[0].body).toContain("needs refactoring");
  }, TEST_TIMEOUT);

  test("comment on removed line has side LEFT", async () => {
    // Use STEP1_PATCH which modifies lines within Step 1 section (non-overview)
    mock = createMockGitHubServer({
      ...defaultConfig(),
      files: [
        {
          filename: "docs/README.md",
          status: "modified",
          patch: STEP1_PATCH,
          content: BASIC_MD,
        },
      ],
    });
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Navigate to Step 1 (which has the diff changes)
    await session.press("j"); // Step 1
    // Switch focus to right pane (diff view)
    await session.press("tab");
    // Display lines for Step 1 range:
    //   [0] context "## Step 1: Auth Middleware"
    //   [1] context " " (empty)
    //   [2] removed "Implement ... pkg/auth..."    ← target
    //   [3] added "Implement ... internal/auth..."
    //   [4] context " " (empty)
    //   [5] context "### 1.1 JWT Verification"
    // Navigate to the removed line (index 2)
    await session.press("j");
    await session.press("j");
    // Add line comment on the removed line
    await session.press("c");
    await session.waitForText("save");
    await session.type("this line was removed");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    // Submit: s → y → ReviewDialog → Comment → ctrl+s
    await session.press("s");
    await session.waitForText("Finish reviewing");
    await session.press("y");
    await session.waitForText("cancel", { timeout: 15000 });
    await session.press("enter"); // Comment
    await session.waitForText("back", { timeout: 5000 });
    await session.press(["ctrl", "s"]);
    await session.waitIdle({ timeout: 10000 });
    // Verify the comment has side=LEFT
    expect(mock.submittedReviews).toHaveLength(1);
    const review = mock.submittedReviews[0];
    expect(review.comments.length).toBeGreaterThanOrEqual(1);
    const comment = review.comments[0];
    expect(comment.side).toBe("LEFT");
    expect(comment.body).toContain("this line was removed");
  }, TEST_TIMEOUT);

  test("approve without comments submits empty review", async () => {
    mock = createMockGitHubServer(defaultConfig());
    session = await launchCommdPR({
      prURL: MOCK_PR_URL,
      mockServerURL: mock.url,
      file: "docs/README.md",
    });
    await session.waitForText("quit", { timeout: 15000 });
    // Submit without comments
    await session.press("s");
    await session.waitForText("Finish reviewing");
    await session.press("y");
    // ReviewDialog: no comments → Approve is first option
    await session.waitForText("cancel", { timeout: 15000 });
    await session.press("enter");
    await session.waitForText("back", { timeout: 5000 });
    await session.press(["ctrl", "s"]);
    await session.waitIdle({ timeout: 10000 });
    expect(mock.submittedReviews).toHaveLength(1);
    expect(mock.submittedReviews[0].event).toBe("APPROVE");
  }, TEST_TIMEOUT);
});
