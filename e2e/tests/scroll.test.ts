import { describe, test, expect, afterEach } from "bun:test";
import { launchCommd, FIXTURE_BASIC, TEST_TIMEOUT } from "../helpers/session";
import type { Session } from "tuistory";

describe("Scroll", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("Ctrl+D scrolls cursor down", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC, rows: 12 });
    // At top, cursor is on Overview
    const before = await session.text({ trimEnd: true });
    expect(before).toMatch(/>\s+Overview/);
    await session.press(["ctrl", "d"]);
    const after = await session.text({ trimEnd: true });
    // Cursor should have moved away from Overview
    expect(after).not.toMatch(/>\s+Overview/);
  }, TEST_TIMEOUT);

  test("Ctrl+U scrolls cursor up", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC, rows: 12 });
    await session.press("G"); // go to bottom
    const atBottom = await session.text({ trimEnd: true });
    expect(atBottom).toMatch(/>\s+S3 Step 3/);
    await session.press(["ctrl", "u"]);
    const afterUp = await session.text({ trimEnd: true });
    // Cursor should have moved away from Step 3
    expect(afterUp).not.toMatch(/>\s+S3 Step 3/);
  }, TEST_TIMEOUT);
});
