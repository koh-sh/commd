import { mkdirSync, writeFileSync, appendFileSync, rmSync } from "fs";
import { resolve, join } from "path";

const PROJECT_ROOT = resolve(import.meta.dir, "../..");

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
  const name = `.tmp-git-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const dir = join(PROJECT_ROOT, "e2e/tests", name);
  mkdirSync(dir);
  git(dir, "init", "-q");
  writeFileSync(join(dir, "doc.md"), ORIGINAL_DOC);
  git(dir, "add", "-A");
  git(dir, "commit", "-q", "--no-verify", "-m", "chore: init");
  if (modify) {
    writeFileSync(join(dir, "doc.md"), ORIGINAL_DOC.replace("# Diff Doc", "# Diff Doc v2"));
    appendFileSync(join(dir, "doc.md"), "Added by diff test\n");
    writeFileSync(join(dir, "new.md"), "# New Doc\n\nFresh file\n");
  }
  return {
    dir,
    cleanup: () => rmSync(dir, { recursive: true, force: true }),
  };
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
