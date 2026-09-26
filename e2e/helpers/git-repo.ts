import { mkdirSync, writeFileSync, rmSync } from "fs";
import { join } from "path";
import { PROJECT_ROOT } from "./paths";

export const ORIGINAL_DOC = [
  "# Diff Doc",
  "",
  "Intro paragraph.",
  "",
  "## Step 1: Alpha",
  "",
  "Alpha body line.",
  "",
  "## Step 2: Beta",
  "",
  "Beta body.",
  "",
].join("\n");

function git(dir: string, ...args: string[]): void {
  const r = Bun.spawnSync(["git", ...args], {
    cwd: dir,
    env: {
      ...process.env,
      GIT_AUTHOR_NAME: "test",
      GIT_AUTHOR_EMAIL: "test@example.com",
      GIT_COMMITTER_NAME: "test",
      GIT_COMMITTER_EMAIL: "test@example.com",
    },
  });
  if (r.exitCode !== 0) {
    throw new Error(`git ${args.join(" ")} failed: ${r.stderr.toString()}`);
  }
}

/**
 * Create a throwaway git repo under e2e/tests with doc.md committed.
 * With modify=true the title line is replaced (one removed + one added line)
 * and a line is appended, and an untracked new.md is added.
 */
export function createRepo(modify: boolean): { dir: string; cleanup: () => void } {
  const modified: Record<string, string> = modify
    ? {
        "doc.md": ORIGINAL_DOC.replace("# Diff Doc", "# Diff Doc v2") + "Added by diff test\n",
        "new.md": "# New Doc\n\nFresh file\n",
      }
    : {};
  return createRepoFrom({ "doc.md": ORIGINAL_DOC }, modified);
}

/**
 * Create a throwaway git repo under e2e/tests with `original` committed and
 * `modified` written over it (file name -> content).
 */
export function createRepoFrom(
  original: Record<string, string>,
  modified: Record<string, string>,
): { dir: string; cleanup: () => void } {
  const name = `.tmp-git-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const dir = join(PROJECT_ROOT, "e2e/tests", name);
  mkdirSync(dir);
  git(dir, "init", "-q");
  for (const [file, content] of Object.entries(original)) writeFileSync(join(dir, file), content);
  git(dir, "add", "-A");
  git(dir, "commit", "-q", "--no-verify", "-m", "chore: init");
  for (const [file, content] of Object.entries(modified)) writeFileSync(join(dir, file), content);
  return {
    dir,
    cleanup: () => rmSync(dir, { recursive: true, force: true }),
  };
}

/**
 * A change that removes the body of section A (a1-a3, old lines 7-9). In the
 * new file, old lines 8 and 9 fall in section B, but the removed lines
 * belong to A, where they were.
 */
export const SECTION_REMOVAL = {
  original: ["# Doc", "", "Intro.", "", "## A", "", "a1", "a2", "a3", "", "## B", "", "b1", ""].join("\n"),
  modified: ["# Doc", "", "Intro.", "", "## A", "", "", "## B", "", "b1", ""].join("\n"),
};
