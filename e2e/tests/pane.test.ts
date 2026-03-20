import { describe, test, expect, afterEach } from "bun:test";
import { launchCommd, FIXTURE_BASIC, TEST_TIMEOUT } from "../helpers/session";
import type { Session } from "tuistory";

/** Count the width of the left pane by finding the first │ in a content row. */
function leftPaneWidth(text: string): number {
  // Find a line inside the pane content (skip title bar)
  for (const line of text.split("\n")) {
    const match = line.match(/^│(.+?)││/);
    if (match) return match[0].indexOf("││");
  }
  return -1;
}

describe("Pane", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("Tab switches focus and back", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("tab"); // switch to right pane
    await session.press("tab"); // switch back to left pane
    await session.press("j");
    const text = await session.text({ trimEnd: true });
    expect(text).toContain("Step 1");
  }, TEST_TIMEOUT);

  test("> widens left pane", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    const before = await session.text({ trimEnd: true });
    const widthBefore = leftPaneWidth(before);
    await session.press(">");
    const after = await session.text({ trimEnd: true });
    const widthAfter = leftPaneWidth(after);
    expect(widthAfter).toBeGreaterThan(widthBefore);
  }, TEST_TIMEOUT);

  test("< narrows left pane", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    const before = await session.text({ trimEnd: true });
    const widthBefore = leftPaneWidth(before);
    await session.press("<");
    const after = await session.text({ trimEnd: true });
    const widthAfter = leftPaneWidth(after);
    expect(widthAfter).toBeLessThan(widthBefore);
  }, TEST_TIMEOUT);
});
