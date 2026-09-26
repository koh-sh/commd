import type { Subprocess } from "bun";
import { PROJECT_ROOT, COMMD_BIN } from "./paths";

export interface WebSession {
  proc: Subprocess<"ignore", "pipe", "pipe">;
  /** Server origin, e.g. http://127.0.0.1:1234 */
  base: string;
  /** Page URL including the session token, as printed by commd. */
  url: string;
  /** Calls the JSON API with the session token. */
  api: (method: string, path: string, body?: unknown) => Promise<Response>;
  /** stderr printed before the URL. */
  log: string;
  /** Resolves to the stderr printed after the URL once commd exits. */
  stderrDone: Promise<string>;
}

/**
 * Start `commd review --web` (without opening a browser, output to stdout)
 * and wait for the URL it prints.
 */
export async function launchWeb(args: string[], cwd = PROJECT_ROOT): Promise<WebSession> {
  const proc = Bun.spawn([COMMD_BIN, "review", ...args, "--web", "--no-open", "--output", "stdout"], {
    cwd,
    stdin: "ignore",
    stdout: "pipe",
    stderr: "pipe",
  });
  const reader = proc.stderr.getReader();
  const decoder = new TextDecoder();
  let log = "";
  let match: RegExpMatchArray | null = null;
  while (!match) {
    const { value, done } = await reader.read();
    if (done) throw new Error(`commd exited before printing the URL:\n${log}`);
    log += decoder.decode(value);
    match = log.match(/(http:\/\/\S+)\/#token=(\w+)/);
  }
  // Keep collecting stderr so it can be checked after commd exits.
  let stderr = "";
  const stderrDone = (async () => {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) return stderr;
      stderr += decoder.decode(value);
    }
  })();
  const [url, base, token] = match;
  const api = (method: string, path: string, body?: unknown) =>
    fetch(base + path, {
      method,
      headers: { "X-Commd-Token": token, "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  return { proc, base, url, api, log, stderrDone };
}

/** Wait for commd to exit and return its exit code, stdout and remaining stderr. */
export async function finished(web: WebSession): Promise<{ code: number; stdout: string; stderr: string }> {
  const code = await web.proc.exited;
  const [stdout, stderr] = await Promise.all([new Response(web.proc.stdout).text(), web.stderrDone]);
  return { code, stdout, stderr };
}

/** Kill commd if it is still running (test cleanup). */
export async function stopWeb(web: WebSession | undefined): Promise<void> {
  if (web && web.proc.exitCode === null) {
    web.proc.kill();
    await web.proc.exited;
  }
}
