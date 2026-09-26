import { beforeAll, afterAll } from "bun:test";
import { chromium, type Browser, type Page } from "playwright";
import { mkdirSync, writeFileSync, rmSync } from "fs";
import { join, resolve } from "path";
import type { WebSession } from "./web";

const PROJECT_ROOT = resolve(import.meta.dir, "../..");

/**
 * Directory for raw JS coverage when COMMD_JS_COVERAGE is set (see
 * scripts/js-coverage.ts and `mise run e2e-cov`).
 */
export const COVERAGE_DIR = join(PROJECT_ROOT, "e2e/.coverage");
const collectCoverage = Boolean(process.env.COMMD_JS_COVERAGE);

let browser: Browser;

/** Launch one headless Chromium for the test file. Call at the top level. */
export function useBrowser(): void {
  beforeAll(async () => {
    browser = await chromium.launch();
  });
  afterAll(async () => {
    await browser?.close();
  });
}

export interface PageOptions {
  width?: number;
  height?: number;
}

/** Open the review page of a web session and wait until it has rendered. */
export async function openPage(web: WebSession, opts: PageOptions = {}): Promise<Page> {
  const page = await browser.newPage({ viewport: { width: opts.width ?? 1280, height: opts.height ?? 800 } });
  // The page asks before closing while the review runs; tests close it anyway.
  page.on("dialog", (d) => d.accept());
  if (collectCoverage) await page.coverage.startJSCoverage({ resetOnNavigation: false });
  await page.goto(web.url);
  await page.waitForSelector("#statusbar, .picker, .done");
  return page;
}

/** Close a page, saving its JS coverage when collecting. */
export async function closePage(page: Page | undefined): Promise<void> {
  if (!page) return;
  if (collectCoverage) {
    const entries = (await page.coverage.stopJSCoverage()).filter((e) => e.url.startsWith("http://127.0.0.1"));
    mkdirSync(COVERAGE_DIR, { recursive: true });
    writeFileSync(join(COVERAGE_DIR, `${Date.now()}-${Math.random().toString(36).slice(2)}.json`), JSON.stringify(entries));
  }
  await page.close();
}

/** Remove collected coverage (start of a coverage run). */
export function resetCoverage(): void {
  rmSync(COVERAGE_DIR, { recursive: true, force: true });
}

/** Press keys in order, like typing them in the TUI. */
export async function press(page: Page, ...keys: string[]): Promise<void> {
  for (const key of keys) await page.keyboard.press(key);
}

/**
 * Retry an assertion until it passes: keys that go through the API update
 * the page asynchronously.
 */
export async function eventually(check: () => Promise<void> | void, timeout = 5000): Promise<void> {
  const end = Date.now() + timeout;
  for (;;) {
    try {
      await check();
      return;
    } catch (e) {
      if (Date.now() > end) throw e;
      await Bun.sleep(25);
    }
  }
}

/**
 * Check that an assertion keeps passing for a while: that a key or event
 * changed nothing, even once the page and the server had time to react.
 */
export async function consistently(check: () => Promise<void> | void, duration = 300): Promise<void> {
  const end = Date.now() + duration;
  do {
    await check();
    await Bun.sleep(25);
  } while (Date.now() < end);
  await check();
}

export const text = (page: Page, selector: string) => page.locator(selector).innerText();
export const count = (page: Page, selector: string) => page.locator(selector).count();
export const activeSection = (page: Page) => page.locator("#sections .active").innerText();
export const cursorLine = (page: Page) => page.locator("#content tr.cursor td.num").innerText();

/** Write a Markdown fixture under e2e/tests and return its path relative to the project root. */
export function writeFixture(content: string): { path: string; cleanup: () => void } {
  const name = `.tmp-web-${Date.now()}-${Math.random().toString(36).slice(2, 8)}.md`;
  const abs = join(PROJECT_ROOT, "e2e/tests", name);
  writeFileSync(abs, content);
  return { path: `e2e/tests/${name}`, cleanup: () => rmSync(abs, { force: true }) };
}
