import { describe, test, expect, afterEach } from "bun:test";
import { resolve } from "path";
import { writeFileSync, readFileSync, rmSync } from "fs";
import type { Session } from "tuistory";
import { launchCommd, addComment, TEST_TIMEOUT } from "../helpers/session";
import { PROJECT_ROOT } from "../helpers/paths";

const ORIGINAL = "# Doc\n\n## Alpha\n\nalpha body\n\n## Beta\n\nbeta line\n";
// A section added above shifts Beta from S2 to S3.
const EDITED = "# Doc\n\n## New\n\nnew body\n\n## Alpha\n\nalpha body\n\n## Beta\n\nbeta line\n";

describe("Reload (R)", () => {
  let session: Session | undefined;
  const doc = resolve(PROJECT_ROOT, "e2e/tests/.tmp-reload.md");
  const out = resolve(PROJECT_ROOT, "e2e/tests/.tmp-reload-output.md");

  afterEach(async () => {
    session?.close();
    session = undefined;
    await Bun.sleep(200); // let commd exit before its files go
    rmSync(doc, { force: true });
    rmSync(out, { force: true });
  });

  test("R reads the edited file, keeping the comment on its section", async () => {
    writeFileSync(doc, ORIGINAL);
    writeFileSync(out, ""); // --output file requires an existing file
    session = await launchCommd({ file: "e2e/tests/.tmp-reload.md", args: ["--output", "file", "--output-path", out] });
    await session.press("j"); // Beta
    await addComment(session, "on beta");

    writeFileSync(doc, EDITED);
    await session.press("R");
    const text = await session.waitForText("Reloaded");
    expect(text).toContain("New");
    expect(text).toContain("S3");

    await session.press("s");
    await session.waitForText("Submit review?");
    await session.press("y");
    await session.waitIdle({ timeout: 5000 });
    expect(readFileSync(out, "utf-8")).toContain("## S3: Beta\n[question] on beta");
  }, TEST_TIMEOUT);

  test("R on an unchanged file says so, and the next key clears it", async () => {
    writeFileSync(doc, ORIGINAL);
    session = await launchCommd({ file: "e2e/tests/.tmp-reload.md" });
    await session.press("R");
    await session.waitForText("File unchanged");
    await session.press("j");
    const text = await session.waitForText("quit");
    expect(text).not.toContain("File unchanged");
  }, TEST_TIMEOUT);
});
