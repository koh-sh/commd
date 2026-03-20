import { describe, test, expect, afterEach } from "bun:test";
import { launchCommd, FIXTURE_BASIC, TEST_TIMEOUT } from "../helpers/session";
import type { Session } from "tuistory";

describe("Navigation", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("initial view shows Overview and sections", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    const text = await session.text();
    expect(text).toContain("Overview");
    expect(text).toContain("Step 1");
    expect(text).toContain("Auth Middleware");
  }, TEST_TIMEOUT);

  test("j moves cursor down", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    const before = await session.text();
    await session.press("j");
    const after = await session.text({ trimEnd: true });
    expect(after).toContain("Step 1");
    expect(after).not.toEqual(before);
  }, TEST_TIMEOUT);

  test("k moves cursor up", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("j");
    const atSecondItem = await session.text({ trimEnd: true });
    await session.press("k");
    const afterK = await session.text({ trimEnd: true });
    expect(afterK).not.toEqual(atSecondItem);
    expect(afterK).toContain("Step 1");
  }, TEST_TIMEOUT);

  test("G goes to bottom", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("G");
    const text = await session.text({ trimEnd: true });
    expect(text).toContain("Step 3");
  }, TEST_TIMEOUT);

  test("gg goes to top", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("G");
    await session.press("g");
    await session.press("g");
    const text = await session.text({ trimEnd: true });
    expect(text).toContain("Overview");
  }, TEST_TIMEOUT);

  test("Enter toggles expand/collapse", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j"); // Step 1 (has children: 1.1, 1.2)
    const before = await session.text();
    expect(before).toContain("JWT Verification");
    await session.press("enter"); // collapse
    const after = await session.text({ trimEnd: true });
    expect(after).not.toContain("JWT Verification");
  }, TEST_TIMEOUT);
});
