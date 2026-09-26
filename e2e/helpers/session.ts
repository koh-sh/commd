import { launchTerminal, type Session } from "tuistory";
import { resolve, join } from "path";
import { unlinkSync, copyFileSync } from "fs";
import { PROJECT_ROOT, COMMD_BIN } from "./paths";

export const TEST_TIMEOUT = 30000;

export interface LaunchOptions {
  /** Markdown file to review. Optional for `--diff`, which can pick files itself. */
  file?: string;
  args?: string[];
  /** Omit the "review" subcommand to exercise the default-command path (commd <file>). */
  withoutSubcommand?: boolean;
  /** Working directory for commd (default: project root). `--diff` tests point this at a git repo. */
  cwd?: string;
  /** Text that marks the TUI as rendered (default: "quit" from the status bar). */
  waitFor?: string;
  cols?: number;
  rows?: number;
  env?: Record<string, string>;
}

export async function launchCommd(opts: LaunchOptions): Promise<Session> {
  const args = [
    ...(opts.withoutSubcommand ? [] : ["review"]),
    ...(opts.file ? [opts.file] : []),
    ...(opts.args ?? []),
  ];

  const session = await launchTerminal({
    command: COMMD_BIN,
    args,
    cols: opts.cols ?? 120,
    rows: opts.rows ?? 36,
    cwd: opts.cwd ?? PROJECT_ROOT,
    env: {
      TERM: "xterm-256color",
      // commd shells out to git for --diff; the PTY env is otherwise empty.
      PATH: process.env.PATH ?? "",
      // Bubble Tea sends ESC[6n (Device Status Report) and ESC]11;? (background
      // color query) on startup and waits up to 5s for a response. tuistory's PTY
      // does not respond to these queries, causing a 5s delay per test. Setting
      // CI=true makes Bubble Tea skip these queries entirely.
      CI: "true",
      ...opts.env,
    },
    waitForData: true,
    waitForDataTimeout: 10000,
  });

  // Wait for Bubble Tea to process WindowSizeMsg and render the TUI
  try {
    await session.waitForText(opts.waitFor ?? "quit", { timeout: 15000 });
  } catch (e) {
    session.close();
    throw e;
  }

  return session;
}

export const FIXTURE_BASIC = "internal/markdown/testdata/basic.md";
export const FIXTURE_CJK_WRAP = "internal/markdown/testdata/cjk-wrap.md";

export const MOCK_PR_URL = "https://github.com/test-owner/test-repo/pull/1";

export interface PRLaunchOptions {
  prURL: string;
  mockServerURL: string;
  file?: string;
  args?: string[];
  cols?: number;
  rows?: number;
  env?: Record<string, string>;
}

/**
 * Launch commd pr subcommand with a mock GitHub API server.
 * Does NOT wait for any specific text — callers must waitForText themselves
 * because the first screen differs (file picker vs review TUI).
 */
export async function launchCommdPR(
  opts: PRLaunchOptions,
): Promise<Session> {
  const args = ["pr", opts.prURL];
  if (opts.file) {
    args.push("--file", opts.file);
  }
  args.push(...(opts.args ?? []));

  const session = await launchTerminal({
    command: COMMD_BIN,
    args,
    cols: opts.cols ?? 120,
    rows: opts.rows ?? 36,
    cwd: PROJECT_ROOT,
    env: {
      TERM: "xterm-256color",
      CI: "true",
      GITHUB_TOKEN: "test-token",
      COMMD_GITHUB_API_URL: opts.mockServerURL,
      ...opts.env,
    },
    waitForData: true,
    waitForDataTimeout: 10000,
  });

  return session;
}

/**
 * Create a temporary copy of a fixture file for tests that need to modify it
 * or produce sidecar files (e.g. .reviewed.json).
 * Returns the relative path (from PROJECT_ROOT) and an async cleanup function.
 *
 * cleanup() waits briefly before deleting to avoid a race condition where
 * session.close() sends SIGTERM and the commd process re-creates the
 * .reviewed.json sidecar during its graceful shutdown (SaveViewedState).
 */
export function createTempFixture(srcRelative: string): {
  path: string;
  cleanup: () => Promise<void>;
} {
  const src = resolve(PROJECT_ROOT, srcRelative);
  const name = `.tmp-${Date.now()}-${Math.random().toString(36).slice(2, 8)}.md`;
  const dst = join(resolve(PROJECT_ROOT, "e2e/tests"), name);
  copyFileSync(src, dst);
  const relPath = `e2e/tests/${name}`;
  return {
    path: relPath,
    cleanup: async () => {
      // Wait for commd process to fully exit after session.close() SIGTERM.
      await Bun.sleep(500);
      try { unlinkSync(dst); } catch {}
      try { unlinkSync(dst + ".reviewed.json"); } catch {}
    },
  };
}

/**
 * Add a comment on the currently selected section.
 * Caller must position the cursor on the target section before calling.
 */
export async function addComment(
  session: Session,
  text: string,
): Promise<void> {
  await session.press("c");
  await session.waitForText("save");
  await session.type(text);
  await session.press(["ctrl", "s"]);
  await session.waitForText("quit");
}
