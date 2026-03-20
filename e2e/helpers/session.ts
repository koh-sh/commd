import { launchTerminal, type Session } from "tuistory";
import { resolve } from "path";

const PROJECT_ROOT = resolve(import.meta.dir, "../..");
const COMMD_BIN = resolve(PROJECT_ROOT, "commd");

export const TEST_TIMEOUT = 30000;

export interface LaunchOptions {
  file: string;
  args?: string[];
  cols?: number;
  rows?: number;
  env?: Record<string, string>;
}

export async function launchCommd(opts: LaunchOptions): Promise<Session> {
  const args = ["review", opts.file, ...(opts.args ?? [])];

  const session = await launchTerminal({
    command: COMMD_BIN,
    args,
    cols: opts.cols ?? 120,
    rows: opts.rows ?? 36,
    cwd: PROJECT_ROOT,
    env: {
      TERM: "xterm-256color",
      ...opts.env,
    },
    waitForData: true,
    waitForDataTimeout: 10000,
  });

  // Wait for Bubble Tea to process WindowSizeMsg and render the TUI
  try {
    await session.waitForText("quit", { timeout: 15000 });
  } catch (e) {
    session.close();
    throw e;
  }

  return session;
}

export const FIXTURE_BASIC = "internal/markdown/testdata/basic.md";

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
