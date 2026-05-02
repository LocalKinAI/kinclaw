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
    ap.add_argument("--js", default="", help="JavaScript expression to evaluate; result returned JSON-stringified")
    ap.add_argument("--timeout-ms", type=int, default=15000)
    args = ap.parse_args()

    if not args.url:
        sys.exit("url required")

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        ctx = browser.new_context(
            user_agent="Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 "
                       "(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
            viewport={"width": 1280, "height": 900},
        )
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
        elif args.screenshot:
            try:
                page.screenshot(path=args.screenshot, type="png", full_page=False)
                print(f"path: {args.screenshot}")
                # marker for kernel image extractor → vision passthrough
                print(f"image://{args.screenshot}")
            except Exception as e:
                sys.exit(f"screenshot failed: {e}")
        else:
            try:
                elem = page.locator(args.selector).first
                text = elem.inner_text(timeout=5000)
                print(text)
            except Exception as e:
                sys.exit(f"extract {args.selector!r} failed: {e}")

        browser.close()


if __name__ == "__main__":
    main()
