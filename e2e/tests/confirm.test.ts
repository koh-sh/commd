import { describe, test, expect, afterEach } from "bun:test";
import { launchCommd, FIXTURE_BASIC, TEST_TIMEOUT } from "../helpers/session";
import type { Session } from "tuistory";

describe("Confirm", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("q shows quit confirmation dialog", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("q");
    const text = await session.waitForText("Quit review?");
    expect(text).toContain("yes");
    expect(text).toContain("no");
  }, TEST_TIMEOUT);

  test("n cancels quit confirmation", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("q");
    await session.waitForText("Quit review?");
    await session.press("n");
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("Overview");
  }, TEST_TIMEOUT);

  test("s shows submit confirmation dialog", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("s");
    const text = await session.waitForText("Submit review?");
    expect(text).toContain("0 comments");
  }, TEST_TIMEOUT);

  test("n cancels submit confirmation", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("s");
    await session.waitForText("Submit review?");
    await session.press("n");
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("Overview");
  }, TEST_TIMEOUT);

  test("esc cancels confirmation dialog", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("q");
    await session.waitForText("Quit review?");
    await session.press("escape");
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("Overview");
  }, TEST_TIMEOUT);
});
