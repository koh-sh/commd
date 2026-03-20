import { describe, test, expect, afterEach } from "bun:test";
import { launchCommd, FIXTURE_BASIC, TEST_TIMEOUT } from "../helpers/session";
import type { Session } from "tuistory";

describe("Viewed", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("v toggles viewed mark on", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j"); // Step 1 (Overview returns nil for Selected())
    await session.press("v");
    const text = await session.text();
    expect(text).toContain("[✓]");
  }, TEST_TIMEOUT);

  test("v again toggles viewed mark off", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("v");
    await session.waitForText("[✓]");
    await session.press("v");
    const text = await session.text();
    expect(text).not.toContain("[✓]");
  }, TEST_TIMEOUT);

  test("viewed mark applies to multiple sections", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j"); // Step 1
    await session.press("v");
    await session.waitForText("[✓]");
    await session.press("j"); // Step 1.1
    await session.press("v");
    const text = await session.text();
    const matches = text.match(/\[✓\]/g);
    expect(matches).not.toBeNull();
    expect(matches!.length).toBe(2);
  }, TEST_TIMEOUT);
});
