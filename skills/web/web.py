#!/usr/bin/env python3
"""
KinClaw web skill — universal Playwright wrapper.

Usage (called via SKILL.md):
  web.py <url> [--wait-for SELECTOR] [--selector SELECTOR]
              [--click SELECTOR] [--type-text TEXT]
              [--screenshot OUT_PATH] [--js JS_CODE]
              [--timeout-ms N]

Single Chromium per invocation: launches → executes the requested
flow → closes. No sidecar, no persistent state. Cold start ~2-3s.

Output:
  - Default: extracted text (the `selector` if given, else <body>).
  - --screenshot OUT_PATH: writes PNG to that path; prints `image://OUT_PATH`
    so kinclaw's marker extractor attaches it for vision-capable brains.
  - --js: JSON-stringified eval result.
"""
import argparse
import json
import os
import sys
from playwright.sync_api import sync_playwright


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("url")
    ap.add_argument("--wait-for", default="", help="CSS selector to await before extracting")
    ap.add_argument("--selector", default="body", help="CSS selector for inner text extraction")
    ap.add_argument("--click", default="", help="CSS selector to click after load (before extract)")
    ap.add_argument("--type-text", default="", help="Text to fill into --click target (must use with --click)")
    ap.add_argument("--press-enter", action="store_true",
                    help="After fill, press Enter on the input — works for React/Vue forms whose submit "
                         "button is gated on internal state. Real keyboard event, not synthetic.")
    ap.add_argument("--screenshot", default="", help="Output path for PNG screenshot of viewport")
    ap.add_argument("--screenshot-selector", default="",
                    help="When set with --screenshot, capture only the bounding box of this CSS selector "
                         "instead of the full viewport. Falls back to viewport screenshot if selector "
                         "is offscreen or zero-sized.")
    ap.add_argument("--screenshot-full-page", action="store_true",
                    help="When set with --screenshot (and no --screenshot-selector), capture the FULL "
                         "scrollable page, not just the viewport. Useful for long articles, infinite "
                         "scroll content captured in one shot.")
    ap.add_argument("--pdf", default="",
                    help="Output path for PDF render of the page. Mutually exclusive with --screenshot. "
                         "Headless Chromium uses its print-pdf engine — high quality, A4 default.")
    ap.add_argument("--pdf-format", default="A4",
                    help="When --pdf is set: page format (A4, Letter, Legal, A3, etc.). Default A4.")
    ap.add_argument("--js", default="", help="JavaScript expression to evaluate; result returned JSON-stringified")
    ap.add_argument("--timeout-ms", type=int, default=15000)
    ap.add_argument("--session-id", default="",
                    help="When set, persist storage state (cookies, localStorage) under "
                         "~/.kinclaw/web-sessions/<session_id>.json. First call writes; subsequent "
                         "calls with same session_id load + reuse. Lets a soul log into a site once "
                         "and run multiple fetches without re-authenticating.")
    ap.add_argument("--upload-selector", default="",
                    help="CSS selector for an <input type=file> to populate via page.set_input_files. "
                         "Pair with --upload-files (comma-separated absolute paths) — the input's "
                         "change/submit fires automatically per Playwright semantics.")
    ap.add_argument("--upload-files", default="",
                    help="Comma-separated absolute file paths to upload via --upload-selector.")
    args = ap.parse_args()

    if not args.url:
        sys.exit("url required")

    # Session persistence: when session_id is set, we look for a saved
    # storage_state.json under ~/.kinclaw/web-sessions/<id>.json and
    # load it into the new context. After the run we save the updated
    # state back so cookies / localStorage / IndexedDB-via-cookies
    # roll forward on subsequent calls with the same id.
    session_state_path = ""
    if args.session_id:
        sessions_dir = os.path.expanduser("~/.kinclaw/web-sessions")
        os.makedirs(sessions_dir, exist_ok=True)
        session_state_path = os.path.join(sessions_dir, f"{args.session_id}.json")

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        ctx_kwargs = {
            "user_agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 "
                          "(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
            "viewport": {"width": 1280, "height": 900},
        }
        if session_state_path and os.path.exists(session_state_path):
            ctx_kwargs["storage_state"] = session_state_path
        ctx = browser.new_context(**ctx_kwargs)
        page = ctx.new_page()

        try:
            page.goto(args.url, wait_until="domcontentloaded", timeout=args.timeout_ms)
        except Exception as e:
            sys.exit(f"goto failed: {e}")

        if args.wait_for:
            try:
                page.wait_for_selector(args.wait_for, timeout=args.timeout_ms, state="visible")
            except Exception as e:
                sys.exit(f"wait_for {args.wait_for!r} failed: {e}")

        # File upload (page.set_input_files). Runs BEFORE click so a
        # form that auto-submits on file selection is captured by the
        # subsequent extract / wait_for. If the upload selector points
        # at a hidden <input type=file>, Playwright handles it.
        if args.upload_selector and args.upload_files:
            file_paths = [
                os.path.expanduser(p.strip())
                for p in args.upload_files.split(",")
                if p.strip()
            ]
            try:
                page.set_input_files(args.upload_selector, file_paths, timeout=10000)
            except Exception as e:
                sys.exit(f"upload to {args.upload_selector!r} failed: {e}")

        if args.click:
            try:
                page.click(args.click, timeout=5000)
            except Exception as e:
                sys.exit(f"click {args.click!r} failed: {e}")

        if args.type_text and args.click:
            try:
                # page.fill() dispatches React-compatible input + change
                # events under the hood — for most React/Vue/Svelte
                # apps this is enough to update the framework's
                # internal state and re-enable a submit button.
                page.fill(args.click, args.type_text, timeout=5000)
            except Exception as e:
                sys.exit(f"type into {args.click!r} failed: {e}")
            # press_enter for forms that don't react to fill alone —
            # some sites listen for keydown specifically (e.g. chat
            # apps that submit on Enter not on click). Real keyboard
            # event = real focus + real key code, not synthetic.
            if args.press_enter:
                try:
                    page.press(args.click, "Enter")
                    # Give the form's submit handler a beat to fire +
                    # any subsequent re-render to settle before we
                    # extract / screenshot.
                    page.wait_for_timeout(500)
                except Exception as e:
                    sys.exit(f"press Enter on {args.click!r} failed: {e}")

        if args.js:
            try:
                result = page.evaluate(args.js)
                print(json.dumps(result, ensure_ascii=False, default=str))
            except Exception as e:
                sys.exit(f"eval failed: {e}")
        elif args.pdf:
            # PDF render. Playwright's page.pdf() requires headless Chromium
            # (which we're running) and writes a self-contained PDF including
            # JS-rendered content + computed CSS. Useful for archival snapshots
            # of articles / dashboards / long-form content.
            try:
                page.pdf(path=args.pdf, format=args.pdf_format, print_background=True)
                print(f"path: {args.pdf}")
                print(f"format: {args.pdf_format}")
            except Exception as e:
                sys.exit(f"pdf failed: {e}")
        elif args.screenshot:
            try:
                if args.screenshot_selector:
                    # Element-only screenshot: locate, scroll into view, then
                    # bounding-box capture. Falls back to viewport screenshot
                    # if the selector is invisible / zero-size — better partial
                    # success than total failure.
                    try:
                        elem = page.locator(args.screenshot_selector).first
                        elem.scroll_into_view_if_needed(timeout=3000)
                        elem.screenshot(path=args.screenshot, type="png")
                        print(f"path: {args.screenshot}")
                        print(f"image://{args.screenshot}")
                        print(f"selector: {args.screenshot_selector}")
                    except Exception as inner:
                        # Fallback to viewport.
                        page.screenshot(path=args.screenshot, type="png", full_page=False)
                        print(f"path: {args.screenshot}")
                        print(f"image://{args.screenshot}")
                        print(f"warning: selector {args.screenshot_selector!r} failed ({inner}); "
                              f"fell back to viewport screenshot")
                else:
                    page.screenshot(
                        path=args.screenshot,
                        type="png",
                        full_page=args.screenshot_full_page,
                    )
                    print(f"path: {args.screenshot}")
                    # marker for kernel image extractor → vision passthrough
                    print(f"image://{args.screenshot}")
                    if args.screenshot_full_page:
                        print("scope: full_page")
                    else:
                        print("scope: viewport")
            except Exception as e:
                sys.exit(f"screenshot failed: {e}")
        else:
            try:
                elem = page.locator(args.selector).first
                text = elem.inner_text(timeout=5000)
                print(text)
            except Exception as e:
                sys.exit(f"extract {args.selector!r} failed: {e}")

        # Save session state BEFORE closing the browser. ctx.storage_state
        # gives us a serializable snapshot of cookies + localStorage +
        # session-storage that we can pass to a future new_context().
        if session_state_path:
            try:
                ctx.storage_state(path=session_state_path)
                print(f"session_saved: {session_state_path}")
            except Exception as e:
                # Don't fail the call — the data was already returned;
                # just warn so the caller knows persistence didn't take.
                print(f"warning: session save failed: {e}", file=sys.stderr)

        browser.close()


if __name__ == "__main__":
    main()
