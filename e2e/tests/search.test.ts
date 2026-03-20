import { describe, test, expect, afterEach } from "bun:test";
import { launchCommd, FIXTURE_BASIC, TEST_TIMEOUT } from "../helpers/session";
import type { Session } from "tuistory";

describe("Search", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("/ opens search and filters sections", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("/");
    await session.type("Step 3");
    const text = await session.text({ trimEnd: true });
    expect(text).toContain("Step 3");
    expect(text).not.toContain("Step 1");
  }, TEST_TIMEOUT);

  test("Enter confirms search and returns to normal mode", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("/");
    await session.type("Step 3");
    await session.press("enter");
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("Step 3");
  }, TEST_TIMEOUT);

  test("Esc cancels search and restores all sections", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("/");
    await session.type("Step 3");
    await session.press("escape");
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("Step 1");
    expect(text).toContain("Step 3");
  }, TEST_TIMEOUT);
});
