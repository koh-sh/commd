import { describe, test, expect, afterEach } from "bun:test";
import {
  launchCommd,
  addComment,
  FIXTURE_BASIC,
  TEST_TIMEOUT,
} from "../helpers/session";
import type { Session } from "tuistory";

describe("Confirm Accept", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("y confirms quit and exits TUI", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("q");
    await session.waitForText("Quit review?");
    await session.press("y");
    // TUI should exit - terminal returns to empty/shell state
    // waitForText("quit") would timeout because the status bar is gone
    await session.waitIdle({ timeout: 5000 });
    const text = await session.text({ immediate: true });
    expect(text).not.toContain("Overview");
  }, TEST_TIMEOUT);

  test("y confirms submit and exits TUI", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("s");
    await session.waitForText("Submit review?");
    await session.press("y");
    await session.waitIdle({ timeout: 5000 });
    const text = await session.text({ immediate: true });
    expect(text).not.toContain("Overview");
  }, TEST_TIMEOUT);

  test("quit with comments shows warning message", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await addComment(session, "test comment");
    // Now quit
    await session.press("q");
    const text = await session.waitForText("Quit without submitting?");
    expect(text).toContain("You have review comments");
  }, TEST_TIMEOUT);
});
