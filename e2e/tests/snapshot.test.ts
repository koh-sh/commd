import { describe, test, expect, afterEach } from "bun:test";
import { launchCommd, FIXTURE_BASIC, TEST_TIMEOUT } from "../helpers/session";
import type { Session } from "tuistory";

describe("Snapshot", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("initial view", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    const text = await session.text({ trimEnd: true });
    expect(text).toMatchSnapshot();
  }, TEST_TIMEOUT);

  test("help overlay", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("?");
    await session.waitForText("Navigation");
    const text = await session.text({ trimEnd: true });
    expect(text).toMatchSnapshot();
  }, TEST_TIMEOUT);

  test("quit confirmation dialog", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("q");
    await session.waitForText("Quit review?");
    const text = await session.text({ trimEnd: true });
    expect(text).toMatchSnapshot();
  }, TEST_TIMEOUT);

  test("search mode", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("/");
    await session.type("Step");
    const text = await session.text({ trimEnd: true });
    expect(text).toMatchSnapshot();
  }, TEST_TIMEOUT);

  test("comment editor", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("c");
    await session.waitForText("save");
    const text = await session.text({ trimEnd: true });
    expect(text).toMatchSnapshot();
  }, TEST_TIMEOUT);

  test("section collapsed", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j"); // Step 1
    await session.press("enter"); // collapse
    const text = await session.text({ trimEnd: true });
    expect(text).toMatchSnapshot();
  }, TEST_TIMEOUT);
});
