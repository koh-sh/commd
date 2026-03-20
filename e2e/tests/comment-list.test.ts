import { describe, test, expect, afterEach } from "bun:test";
import {
  launchCommd,
  addComment,
  FIXTURE_BASIC,
  TEST_TIMEOUT,
} from "../helpers/session";
import type { Session } from "tuistory";

describe("Comment List", () => {
  let session: Session;

  afterEach(() => {
    session?.close();
  });

  test("C opens comment list for section with comments", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await addComment(session, "first comment");
    await session.press("C");
    const text = await session.waitForText("Comments on");
    expect(text).toContain("#1");
    expect(text).toContain("first comment");
  }, TEST_TIMEOUT);

  test("e edits existing comment", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await addComment(session, "original text");
    await session.press("C");
    await session.waitForText("Comments on");
    await session.press("e"); // edit selected comment
    await session.waitForText("save");
    // Editor should contain original text
    const editorText = await session.text();
    expect(editorText).toContain("original text");
  }, TEST_TIMEOUT);

  test("d deletes a comment", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await addComment(session, "to be deleted");
    await session.press("C");
    await session.waitForText("Comments on");
    await session.press("d");
    await session.press("escape");
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).not.toContain("[*]");
  }, TEST_TIMEOUT);

  test("multiple comments shown in list", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await addComment(session, "comment one");
    await addComment(session, "comment two");
    await session.press("C");
    const text = await session.waitForText("Comments on");
    expect(text).toContain("#1");
    expect(text).toContain("#2");
    expect(text).toContain("comment one");
    expect(text).toContain("comment two");
  }, TEST_TIMEOUT);

  test("j/k navigates between comments in list", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await addComment(session, "first");
    await addComment(session, "second");
    await session.press("C");
    await session.waitForText("Comments on");
    // Initial: first comment selected (> #1)
    const before = await session.text();
    await session.press("j"); // select second
    const after = await session.text();
    expect(after).not.toEqual(before);
  }, TEST_TIMEOUT);

  test("Esc returns from comment list to normal mode", async () => {
    session = await launchCommd({ file: FIXTURE_BASIC });
    await session.press("j");
    await addComment(session, "test");
    await session.press("C");
    await session.waitForText("Comments on");
    await session.press("escape");
    await session.waitForText("quit");
    const text = await session.text();
    expect(text).toContain("Overview");
  }, TEST_TIMEOUT);
});
