# E2E Tests

E2E tests in `e2e/` use tuistory (terminal UI automation) with bun:test:

- `launchCommd()` helper spawns the built binary in a virtual terminal and waits for Bubble Tea to render
- Each test runs with a fixed terminal size (120×36 by default)
- Snapshot tests (`snapshot.test.ts`) capture full screen state for layout regression detection
- Fixtures reuse `internal/markdown/testdata/basic.md`
- `addComment()` shared helper for comment creation sequences
- `review --web` is tested at two levels: `web.test.ts` drives the JSON API over HTTP, and `web-ui*.test.ts` drive the page in headless Chromium via Playwright (the `playwright` library under bun:test, not the Playwright test runner). Like the TUI tests, the critical paths (`web-ui.test.ts`) are in the basic tier and the rest (`web-ui-keys` / `web-ui-modes` / `web-ui-edge`) only in the full suite, which aims to cover every key binding and mode. Helpers: `launchWeb()` (`helpers/web.ts`), `useBrowser()` / `openPage()` / `eventually()` (`helpers/browser.ts`; wrap assertions after server-bound keys in `eventually()` since the page updates asynchronously). `mise run e2e-cov` collects V8 coverage of the page scripts (`COMMD_JS_COVERAGE=1`) and prints it per file (`bun scripts/js-coverage.ts --uncovered` lists missed lines). The e2e tasks run `bunx playwright install chromium` first
