import { describe, test, expect, afterEach } from "bun:test";
import { launchCommd, FIXTURE_BASIC, TEST_TIMEOUT } from "../helpers/session";
import type { Session } from "tuistory";

describe("Comment", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("c opens comment editor with default label [question]", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("c");
    const text = await session.waitForText("save");
    expect(text).toContain("label: question");
  }, TEST_TIMEOUT);

  test("type and Ctrl+S saves comment", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("c");
    await session.waitForText("save");
    await session.type("Fix the auth middleware");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("[*]");
    expect(text).toContain("Fix the auth middleware");
  }, TEST_TIMEOUT);

  test("Esc cancels comment without saving", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("c");
    await session.waitForText("save");
    await session.type("will be cancelled");
    await session.press("escape");
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).not.toContain("[*]");
  }, TEST_TIMEOUT);

  test("Tab cycles label from question to suggestion", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("c");
    await session.waitForText("label: question");
    await session.press("tab");
    // question → nitpick (next in cycle)
    const text = await session.text();
    expect(text).toContain("label: nitpick");
  }, TEST_TIMEOUT);

  test("Ctrl+D cycles decoration", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("c");
    await session.waitForText("save");
    // Default decoration is "none" (not shown in label)
    // Ctrl+D cycles: none → non-blocking → blocking → if-minor → none
    await session.press(["ctrl", "d"]);
    const text = await session.text();
    expect(text).toContain("non-blocking");
  }, TEST_TIMEOUT);

  test("saved comment shows correct label in detail pane", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await session.press("c");
    await session.waitForText("save");
    await session.press("tab"); // question → nitpick
    await session.type("minor formatting");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("[nitpick]");
    expect(text).toContain("minor formatting");
  }, TEST_TIMEOUT);

  test("comments on multiple sections", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    // Comment on Step 1
    await session.press("j");
    await session.press("c");
    await session.waitForText("save");
    await session.type("step 1 comment");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    // Comment on Step 2
    await session.press("j"); // 1.1
    await session.press("j"); // 1.2
    await session.press("j"); // Step 2
    await session.press("c");
    await session.waitForText("save");
    await session.type("step 2 comment");
    await session.press(["ctrl", "s"]);
    await session.waitForText("quit");
    const text = await session.text();
    // Both sections should have badges
    const badges = text.match(/\[\*\]/g);
    expect(badges).not.toBeNull();
    expect(badges!.length).toBeGreaterThanOrEqual(2);
  }, TEST_TIMEOUT);
});
