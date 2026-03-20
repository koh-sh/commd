import { describe, test, expect, afterEach } from "bun:test";
import { launchCommd, FIXTURE_BASIC, TEST_TIMEOUT } from "../helpers/session";
import type { Session } from "tuistory";

describe("Help", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("? shows help overlay with keybinding info", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("?");
    const text = await session.waitForText("Navigation");
    expect(text).toContain("Move cursor up/down");
    expect(text).toContain("Comment Editor");
  }, TEST_TIMEOUT);

  test("? again closes help and returns to normal mode", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("?");
    await session.waitForText("Navigation");
    await session.press("?");
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("Overview");
    expect(text).not.toContain("Navigation");
  }, TEST_TIMEOUT);

  test("Esc closes help", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("?");
    await session.waitForText("Navigation");
    await session.press("escape");
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("Overview");
    expect(text).not.toContain("Navigation");
  }, TEST_TIMEOUT);
});
