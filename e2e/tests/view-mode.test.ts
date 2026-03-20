import { describe, test, expect, afterEach } from "bun:test";
import { launchCommd, FIXTURE_BASIC, TEST_TIMEOUT } from "../helpers/session";
import type { Session } from "tuistory";

describe("View Mode", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("f toggles to full view", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    // Default: section view, status bar shows "f full"
    const before = await session.text();
    expect(before).toContain("f full");
    await session.press("f");
    // Full view: status bar shows "f section"
    const after = await session.text();
    expect(after).toContain("f section");
  }, TEST_TIMEOUT);

  test("f again toggles back to section view", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("f");
    await session.waitForText("f section");
    await session.press("f");
    const text = await session.text();
    expect(text).toContain("f full");
  }, TEST_TIMEOUT);

  test("r toggles raw source view with line numbers", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j"); // Select Step 1 (has content)
    await session.press("r");
    const text = await session.text();
    // Raw view shows line number gutter
    expect(text).toMatch(/\d+\s*│/);
    // Status bar shows render toggle and line position
    expect(text).toContain("r render");
  }, TEST_TIMEOUT);

  test("r again toggles back to rendered view", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("r");
    await session.waitForText("r render");
    await session.press("r");
    const text = await session.text();
    expect(text).toContain("r raw");
  }, TEST_TIMEOUT);
});
