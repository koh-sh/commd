import { describe, test, expect, afterEach } from "bun:test";
import { resolve } from "path";
import { existsSync } from "fs";
import { TEST_TIMEOUT, FIXTURE_BASIC, createTempFixture } from "../helpers/session";
import { createRepo } from "../helpers/git-repo";
import { launchWeb, finished, stopWeb, type WebSession } from "../helpers/web";

// The browser UI is served by `commd review --web`. These tests drive the
// JSON API over HTTP the way the page's script does, and check what the
// command hands back. web-ui.test.ts drives the page itself in a browser.

const PROJECT_ROOT = resolve(import.meta.dir, "../..");

describe("Web Review", () => {
  let web: WebSession | undefined;
  let repo: { dir: string; cleanup: () => void } | undefined;
  let fixture: { path: string; cleanup: () => Promise<void> } | undefined;

  afterEach(async () => {
    await stopWeb(web);
    web = undefined;
    repo?.cleanup();
    repo = undefined;
    await fixture?.cleanup();
    fixture = undefined;
  });

  test(
    "serves the page and hands the submitted comments back",
    async () => {
      web = await launchWeb([FIXTURE_BASIC]);

      const page = await fetch(web.base + "/");
      expect(page.status).toBe(200);
      expect(await page.text()).toContain("main.js");
      // The API needs the token from the printed URL.
      expect((await fetch(web.base + "/api/state")).status).toBe(401);

      const state = await (await web.api("GET", "/api/state")).json();
      expect(state.phase).toBe("review");
      expect(state.theme).toBe("dark");
      expect(state.file.sections.map((s: { id: string }) => s.id)).toEqual([
        "overview", "S1", "S1.1", "S1.2", "S2", "S2.1", "S2.2", "S3",
      ]);

      let res = await web.api("POST", `/api/files/${state.seq}/comments`, {
        sectionId: "S1.1", action: "issue", decoration: "blocking", body: "Key rotation is undefined",
      });
      expect(res.status).toBe(200);
      res = await web.api("POST", `/api/files/${state.seq}/comments`, {
        action: "question", decoration: "", body: "Which router?", startLine: 15, endLine: 15,
      });
      expect(res.status).toBe(200);
      res = await web.api("POST", `/api/files/${state.seq}/finish`, { action: "submit" });
      expect((await res.json()).phase).toBe("done");

      const { code, stdout } = await finished(web);
      expect(code).toBe(0);
      expect(stdout).toContain("## S1.1: 1.1 JWT Verification\n[issue (blocking)] Key rotation is undefined");
      expect(stdout).toContain("`L15` [question] Which router?\n> Register the middleware in the HTTP router.");
    },
    TEST_TIMEOUT,
  );

  test(
    "quit in the browser outputs nothing",
    async () => {
      web = await launchWeb([FIXTURE_BASIC, "--theme", "light"]);
      const state = await (await web.api("GET", "/api/state")).json();
      expect(state.theme).toBe("light");
      await web.api("POST", `/api/files/${state.seq}/comments`, { sectionId: "S1", action: "note", decoration: "", body: "dropped" });
      await web.api("POST", `/api/files/${state.seq}/finish`, { action: "quit" });

      const { code, stdout } = await finished(web);
      expect(code).toBe(0);
      expect(stdout).toBe("");
    },
    TEST_TIMEOUT,
  );

  test(
    "--track-viewed persists and restores viewed marks",
    async () => {
      fixture = createTempFixture(FIXTURE_BASIC);
      web = await launchWeb([fixture.path, "--track-viewed"]);
      let state = await (await web.api("GET", "/api/state")).json();
      const res = await web.api("PUT", `/api/files/${state.seq}/viewed/S1.1`, { viewed: true });
      expect(res.status).toBe(200);
      await web.api("POST", `/api/files/${state.seq}/finish`, { action: "quit" });
      expect((await finished(web)).code).toBe(0);
      expect(existsSync(resolve(PROJECT_ROOT, fixture.path + ".reviewed.json"))).toBe(true);

      // A new session starts with the mark restored.
      web = await launchWeb([fixture.path, "--track-viewed"]);
      state = await (await web.api("GET", "/api/state")).json();
      expect(state.file.viewed).toEqual(["S1.1"]);
    },
    TEST_TIMEOUT,
  );

  test(
    "--diff picks files, then reviews them one by one",
    async () => {
      repo = createRepo(true);
      web = await launchWeb(["--diff"], repo.dir);

      // Like the TUI, the changed files are offered in a picker first.
      let state = await (await web.api("GET", "/api/state")).json();
      expect(state.phase).toBe("pick");
      expect([...state.pick].sort()).toEqual(["doc.md", "new.md"]);
      state = await (await web.api("POST", "/api/pick", { paths: state.pick })).json();
      expect(state.multi).toBe(true);

      // Comment on doc.md and finish it; skip new.md.
      const reviewed: string[] = [];
      while (state.phase === "review") {
        reviewed.push(state.file.path);
        expect(state.file.diff).toBe(true);
        if (state.file.path === "doc.md") {
          const removed = state.file.lines.find((l: { type: string }) => l.type === "-");
          expect(removed).toMatchObject({ text: "# Diff Doc", line: 1, side: "LEFT" });
          const res = await web.api("POST", `/api/files/${state.seq}/comments`, {
            action: "question", decoration: "", body: "Why rename?", startLine: 1, side: "LEFT",
          });
          expect(res.status).toBe(200);
        }
        const action = state.file.path === "doc.md" ? "submit" : "quit";
        state = await (await web.api("POST", `/api/files/${state.seq}/finish`, { action })).json();
      }
      expect(reviewed.sort()).toEqual(["doc.md", "new.md"]);

      const { stdout } = await finished(web);
      expect(stdout).toContain("on: doc.md");
      expect(stdout).toContain("`L1 (removed)` [question] Why rename?\n> # Diff Doc");
      expect(stdout).not.toContain("new.md");
    },
    TEST_TIMEOUT,
  );

  test(
    "cancelling the picker outputs nothing",
    async () => {
      repo = createRepo(true);
      web = await launchWeb(["--diff"], repo.dir);
      const res = await web.api("POST", "/api/pick", { cancel: true });
      expect((await res.json()).phase).toBe("done");
      const { code, stdout } = await finished(web);
      expect(code).toBe(0);
      expect(stdout).toBe("");
    },
    TEST_TIMEOUT,
  );
});
