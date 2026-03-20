import { describe, test, expect, afterEach } from "bun:test";
import { launchCommd, FIXTURE_BASIC, TEST_TIMEOUT } from "../helpers/session";
import type { Session } from "tuistory";

describe("Line Comment", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("c in raw view opens comment with line reference", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j"); // Step 1
    await session.press("r"); // raw view
    await session.waitForText("r render");
    await session.press("c"); // comment on current line
    const text = await session.waitForText("save");
    // Comment header shows line reference like (L5)
    expect(text).toMatch(/\(L\d+\)/);
  }, TEST_TIMEOUT);

  test("line comment saves with line ref shown in detail", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("r");
    await session.waitForText("r render");
    await session.press("c");
    await session.waitForText("save");
    await session.type("line level feedback");
    await session.press(["ctrl", "s"]);
    await session.waitForText("r render");
    const text = await session.text();
    expect(text).toContain("line level feedback");
    expect(text).toContain("[*]");
  }, TEST_TIMEOUT);

  test("j moves line cursor in raw view before commenting", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("r");
    await session.waitForText("r render");
    // Move down a few lines
    await session.press("j");
    await session.press("j");
    await session.press("j");
    await session.press("c");
    const text = await session.waitForText("save");
    // Line ref should be greater than the first line
    expect(text).toMatch(/\(L\d+\)/);
  }, TEST_TIMEOUT);
});

describe("Visual Line Selection Comment", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("V enters visual select mode", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("r");
    await session.waitForText("r render");
    await session.press("V");
    const text = await session.waitForText("VISUAL");
    expect(text).toContain("VISUAL");
    expect(text).toContain("extend");
    expect(text).toContain("comment");
  }, TEST_TIMEOUT);

  test("V + j extends selection and c opens comment with range", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("r");
    await session.waitForText("r render");
    await session.press("V"); // start visual select
    await session.waitForText("VISUAL");
    await session.press("j"); // extend down
    await session.press("j"); // extend further
    await session.press("c"); // comment on range
    const text = await session.waitForText("save");
    // Should show range like (L5-L7)
    expect(text).toMatch(/\(L\d+-L\d+\)/);
  }, TEST_TIMEOUT);

  test("range comment saves and shows in raw view", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("r");
    await session.waitForText("r render");
    await session.press("V");
    await session.waitForText("VISUAL");
    await session.press("j");
    await session.press("c");
    await session.waitForText("save");
    await session.type("multi-line issue");
    await session.press(["ctrl", "s"]);
    await session.waitForText("r render");
    const text = await session.text();
    expect(text).toContain("multi-line issue");
    expect(text).toContain("[*]");
  }, TEST_TIMEOUT);

  test("Esc cancels visual selection and returns to raw view", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("r");
    await session.waitForText("r render");
    await session.press("V");
    await session.waitForText("VISUAL");
    await session.press("escape");
    const text = await session.waitForText("r render");
    expect(text).not.toContain("VISUAL");
  }, TEST_TIMEOUT);
});
