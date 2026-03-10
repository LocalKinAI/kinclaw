#!/usr/bin/env python3
"""NotebookLM browser automation skill for LocalKin.

Connects to an existing Chrome instance via CDP to manage
Google NotebookLM notebooks (CRUD + source management).

Requires: pip install playwright && playwright install chromium
Environment: CHROME_REMOTE_URL=ws://127.0.0.1:9222
"""

import argparse
import json
import os
import re
import sys
import time

try:
    from playwright.sync_api import sync_playwright, TimeoutError as PWTimeout
except ImportError:
    print("Error: playwright not installed. Run: pip install playwright && playwright install chromium")
    sys.exit(1)

BASE_URL = "https://notebooklm.google.com"
CHROME_URL = os.environ.get("CHROME_REMOTE_URL", "")


# ---------------------------------------------------------------------------
# Browser helpers
# ---------------------------------------------------------------------------

def connect():
    """Connect to existing Chrome via CDP or fail with clear message."""
    if not CHROME_URL:
        print("Error: CHROME_REMOTE_URL not set.")
        print("Launch Chrome with: google-chrome --remote-debugging-port=9222")
        print("Then: export CHROME_REMOTE_URL='ws://127.0.0.1:9222'")
        sys.exit(1)
    # Strip path — playwright wants just ws://host:port
    ws_url = re.sub(r"(/devtools/browser/.*)?$", "", CHROME_URL)
    pw = sync_playwright().start()
    try:
        browser = pw.chromium.connect_over_cdp(ws_url)
    except Exception as e:
        pw.stop()
        print(f"Error: cannot connect to Chrome at {ws_url}: {e}")
        sys.exit(1)
    return pw, browser


def get_page(browser):
    """Get or create a page in the default context."""
    ctx = browser.contexts[0] if browser.contexts else browser.new_context()
    page = ctx.new_page()
    return page


def go_home(page):
    """Navigate to NotebookLM home and wait for notebooks to load."""
    page.goto(BASE_URL, wait_until="networkidle", timeout=30000)
    # Wait for the page to be interactive
    page.wait_for_load_state("domcontentloaded")
    time.sleep(2)  # Allow JS rendering


def extract_notebook_id(ref):
    """Extract notebook ID from a full URL or raw ID string."""
    if "/" in ref:
        # https://notebooklm.google.com/notebook/<id>
        match = re.search(r"/notebook/([a-f0-9-]+)", ref)
        if match:
            return match.group(1)
    return ref.strip()


# ---------------------------------------------------------------------------
# Actions
# ---------------------------------------------------------------------------

def action_list(page):
    """List all notebooks on the home page."""
    go_home(page)
    notebooks = _scrape_notebook_list(page)
    if not notebooks:
        print("No notebooks found.")
        return
    print(json.dumps(notebooks, indent=2, ensure_ascii=False))


def action_create(page, name):
    """Create a new notebook."""
    if not name:
        print("Error: --name is required for create")
        sys.exit(1)
    go_home(page)

    # Click "New notebook" / "Create" button
    create_btn = _find_create_button(page)
    if not create_btn:
        print("Error: cannot find 'New notebook' button on page")
        sys.exit(1)
    create_btn.click()
    time.sleep(3)

    # We should now be inside a new notebook
    # Try to set the title
    current_url = page.url
    nb_id = ""
    match = re.search(r"/notebook/([a-f0-9-]+)", current_url)
    if match:
        nb_id = match.group(1)

    # Click on title area and set name
    _set_notebook_title(page, name)

    result = {"status": "created", "name": name}
    if nb_id:
        result["id"] = nb_id
        result["url"] = f"{BASE_URL}/notebook/{nb_id}"
    print(json.dumps(result, indent=2, ensure_ascii=False))


def action_find(page, query):
    """Find notebooks matching a query string."""
    if not query:
        print("Error: --query is required for find")
        sys.exit(1)
    go_home(page)
    notebooks = _scrape_notebook_list(page)
    query_lower = query.lower()
    matches = [nb for nb in notebooks if query_lower in nb.get("name", "").lower()]
    if not matches:
        print(f"No notebooks matching '{query}'.")
        return
    print(json.dumps(matches, indent=2, ensure_ascii=False))


def action_read(page, notebook_id):
    """Read a notebook's details and list its sources."""
    if not notebook_id:
        print("Error: --notebook-id is required for read")
        sys.exit(1)
    nb_id = extract_notebook_id(notebook_id)
    page.goto(f"{BASE_URL}/notebook/{nb_id}", wait_until="networkidle", timeout=30000)
    time.sleep(3)

    # Scrape notebook info
    title = _get_notebook_title(page)
    sources = _get_notebook_sources(page)
    result = {
        "id": nb_id,
        "url": f"{BASE_URL}/notebook/{nb_id}",
        "title": title,
        "sources": sources,
    }
    print(json.dumps(result, indent=2, ensure_ascii=False))


def action_delete(page, notebook_id, confirm):
    """Delete a notebook (requires confirm='yes')."""
    if not notebook_id:
        print("Error: --notebook-id is required for delete")
        sys.exit(1)
    if confirm != "yes":
        nb_id = extract_notebook_id(notebook_id)
        print(json.dumps({
            "status": "pending_confirmation",
            "id": nb_id,
            "message": "To delete this notebook, call again with confirm='yes'. This cannot be undone.",
        }, indent=2))
        return

    nb_id = extract_notebook_id(notebook_id)
    go_home(page)
    time.sleep(2)

    # Find the notebook card and trigger delete via its menu
    deleted = _delete_notebook_from_home(page, nb_id)
    if deleted:
        print(json.dumps({"status": "deleted", "id": nb_id}, indent=2))
    else:
        print(f"Error: could not find or delete notebook {nb_id}")
        sys.exit(1)


def action_add_source(page, notebook_id, source):
    """Add a source (file or URL) to a notebook."""
    if not notebook_id:
        print("Error: --notebook-id is required for add_source")
        sys.exit(1)
    if not source:
        print("Error: --source is required for add_source")
        sys.exit(1)

    nb_id = extract_notebook_id(notebook_id)
    page.goto(f"{BASE_URL}/notebook/{nb_id}", wait_until="networkidle", timeout=30000)
    time.sleep(3)

    if source.startswith("http://") or source.startswith("https://"):
        _add_url_source(page, source)
    elif os.path.isfile(source):
        _add_file_source(page, source)
    else:
        print(f"Error: source '{source}' is not a valid URL or file path")
        sys.exit(1)

    time.sleep(3)
    sources = _get_notebook_sources(page)
    print(json.dumps({
        "status": "source_added",
        "id": nb_id,
        "source": source,
        "total_sources": len(sources),
    }, indent=2, ensure_ascii=False))


# ---------------------------------------------------------------------------
# DOM interaction helpers
# ---------------------------------------------------------------------------

def _scrape_notebook_list(page):
    """Scrape notebook cards from the home page."""
    notebooks = []
    # NotebookLM renders notebook cards — try multiple selector strategies
    for selector in [
        'a[href*="/notebook/"]',
        '[data-notebook-id]',
        '.notebook-card',
    ]:
        try:
            elements = page.query_selector_all(selector)
            if elements:
                for el in elements:
                    href = el.get_attribute("href") or ""
                    nb_id = ""
                    match = re.search(r"/notebook/([a-f0-9-]+)", href)
                    if match:
                        nb_id = match.group(1)
                    data_id = el.get_attribute("data-notebook-id") or ""
                    name = el.inner_text().strip().split("\n")[0]
                    nb = {"name": name}
                    if nb_id:
                        nb["id"] = nb_id
                        nb["url"] = f"{BASE_URL}/notebook/{nb_id}"
                    elif data_id:
                        nb["id"] = data_id
                        nb["url"] = f"{BASE_URL}/notebook/{data_id}"
                    notebooks.append(nb)
                break
        except Exception:
            continue

    # Deduplicate by id
    seen = set()
    unique = []
    for nb in notebooks:
        key = nb.get("id", nb.get("name"))
        if key and key not in seen:
            seen.add(key)
            unique.append(nb)
    return unique


def _find_create_button(page):
    """Find the 'New notebook' / 'Create' button."""
    for strategy in [
        lambda: page.get_by_role("button", name=re.compile(r"new|create", re.I)),
        lambda: page.locator('button:has-text("New")'),
        lambda: page.locator('button:has-text("Create")'),
        lambda: page.locator('[aria-label*="new" i], [aria-label*="create" i]'),
        lambda: page.locator('button.create-button, button.new-notebook'),
    ]:
        try:
            btn = strategy()
            if btn.count() > 0:
                return btn.first
        except Exception:
            continue
    return None


def _set_notebook_title(page, name):
    """Set the notebook title after creation."""
    for selector in [
        '[contenteditable="true"]',
        'input[aria-label*="title" i]',
        'h1[contenteditable]',
        '.notebook-title',
    ]:
        try:
            el = page.locator(selector).first
            if el.is_visible(timeout=3000):
                el.click()
                el.fill("") if el.evaluate("e => e.tagName") == "INPUT" else None
                page.keyboard.select_all()
                page.keyboard.type(name, delay=30)
                page.keyboard.press("Enter")
                return
        except Exception:
            continue


def _get_notebook_title(page):
    """Get the current notebook's title."""
    for selector in [
        'h1', '[contenteditable="true"]',
        '.notebook-title', '[aria-label*="title" i]',
    ]:
        try:
            el = page.locator(selector).first
            if el.is_visible(timeout=2000):
                text = el.inner_text().strip()
                if text:
                    return text
        except Exception:
            continue
    return page.title()


def _get_notebook_sources(page):
    """List sources in the current notebook."""
    sources = []
    for selector in [
        '.source-item', '[data-source-id]',
        'div[role="listitem"]', '.source-card',
    ]:
        try:
            elements = page.query_selector_all(selector)
            if elements:
                for el in elements:
                    name = el.inner_text().strip().split("\n")[0]
                    source_id = el.get_attribute("data-source-id") or ""
                    src = {"name": name}
                    if source_id:
                        src["id"] = source_id
                    sources.append(src)
                break
        except Exception:
            continue
    return sources


def _add_url_source(page, url):
    """Add a URL as a source to the current notebook."""
    # Look for "Add source" button
    add_btn = _find_add_source_button(page)
    if not add_btn:
        print("Error: cannot find 'Add source' button")
        sys.exit(1)
    add_btn.click()
    time.sleep(2)

    # Look for URL/website option in the dialog
    for label in ["Website", "URL", "Link", "Web"]:
        try:
            option = page.get_by_text(label, exact=False)
            if option.count() > 0:
                option.first.click()
                time.sleep(1)
                break
        except Exception:
            continue

    # Fill in URL input
    for selector in [
        'input[type="url"]', 'input[type="text"]',
        'input[placeholder*="URL" i]', 'input[placeholder*="http" i]',
        'input[aria-label*="URL" i]',
    ]:
        try:
            inp = page.locator(selector).first
            if inp.is_visible(timeout=2000):
                inp.fill(url)
                time.sleep(1)
                break
        except Exception:
            continue

    # Click submit/insert
    for btn_text in ["Insert", "Add", "Submit", "Upload"]:
        try:
            btn = page.get_by_role("button", name=re.compile(btn_text, re.I))
            if btn.count() > 0:
                btn.first.click()
                time.sleep(3)
                return
        except Exception:
            continue


def _add_file_source(page, filepath):
    """Upload a file as a source to the current notebook."""
    abs_path = os.path.abspath(filepath)
    add_btn = _find_add_source_button(page)
    if not add_btn:
        print("Error: cannot find 'Add source' button")
        sys.exit(1)
    add_btn.click()
    time.sleep(2)

    # Look for upload/file option
    for label in ["Upload", "PDF", "File", "Document"]:
        try:
            option = page.get_by_text(label, exact=False)
            if option.count() > 0:
                option.first.click()
                time.sleep(1)
                break
        except Exception:
            continue

    # Handle file chooser
    try:
        with page.expect_file_chooser(timeout=5000) as fc_info:
            # Click the upload area or browse button
            for sel in ['input[type="file"]', 'button:has-text("Browse")', '.upload-area']:
                try:
                    el = page.locator(sel).first
                    if el.is_visible(timeout=1000):
                        el.click()
                        break
                except Exception:
                    continue
        file_chooser = fc_info.value
        file_chooser.set_files(abs_path)
        time.sleep(5)  # Wait for upload processing
    except PWTimeout:
        # Try direct file input approach
        file_input = page.locator('input[type="file"]')
        if file_input.count() > 0:
            file_input.first.set_input_files(abs_path)
            time.sleep(5)
        else:
            print("Error: could not find file upload mechanism")
            sys.exit(1)


def _find_add_source_button(page):
    """Find the 'Add source' button inside a notebook."""
    for strategy in [
        lambda: page.get_by_role("button", name=re.compile(r"add.*source", re.I)),
        lambda: page.locator('button:has-text("Add source")'),
        lambda: page.locator('button:has-text("Add")').first,
        lambda: page.locator('[aria-label*="add source" i]'),
        lambda: page.locator('.add-source-button'),
    ]:
        try:
            btn = strategy()
            if btn.count() > 0:
                return btn.first
        except Exception:
            continue
    return None


def _delete_notebook_from_home(page, nb_id):
    """Delete a notebook from the home page by its ID."""
    # Find the notebook card
    card = None
    for selector in [
        f'a[href*="/notebook/{nb_id}"]',
        f'[data-notebook-id="{nb_id}"]',
    ]:
        try:
            el = page.locator(selector).first
            if el.is_visible(timeout=3000):
                card = el
                break
        except Exception:
            continue

    if not card:
        return False

    # Right-click or find the menu button on the card
    try:
        # Try 3-dot menu first
        menu_btn = card.locator('button[aria-label*="more" i], button[aria-label*="menu" i], .menu-button').first
        if menu_btn.is_visible(timeout=2000):
            menu_btn.click()
        else:
            card.click(button="right")
    except Exception:
        card.click(button="right")

    time.sleep(1)

    # Click "Delete" in the context menu
    for label in ["Delete", "Remove", "Move to trash"]:
        try:
            delete_opt = page.get_by_role("menuitem", name=re.compile(label, re.I))
            if delete_opt.count() > 0:
                delete_opt.first.click()
                time.sleep(1)
                break
        except Exception:
            try:
                delete_opt = page.get_by_text(label, exact=False)
                if delete_opt.count() > 0:
                    delete_opt.first.click()
                    time.sleep(1)
                    break
            except Exception:
                continue

    # Confirm deletion dialog if it appears
    time.sleep(1)
    for btn_text in ["Delete", "Confirm", "Yes", "OK"]:
        try:
            confirm_btn = page.get_by_role("button", name=re.compile(f"^{btn_text}$", re.I))
            if confirm_btn.count() > 0:
                confirm_btn.first.click()
                time.sleep(2)
                return True
        except Exception:
            continue

    return True


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(description="NotebookLM automation")
    parser.add_argument("--action", required=True,
                        choices=["list", "create", "find", "read", "delete", "add_source"])
    parser.add_argument("--name", default="")
    parser.add_argument("--query", default="")
    parser.add_argument("--notebook-id", default="")
    parser.add_argument("--source", default="")
    parser.add_argument("--confirm", default="")
    args = parser.parse_args()

    pw, browser = connect()
    try:
        page = get_page(browser)
        if args.action == "list":
            action_list(page)
        elif args.action == "create":
            action_create(page, args.name)
        elif args.action == "find":
            action_find(page, args.query)
        elif args.action == "read":
            action_read(page, args.notebook_id)
        elif args.action == "delete":
            action_delete(page, args.notebook_id, args.confirm)
        elif args.action == "add_source":
            action_add_source(page, args.notebook_id, args.source)
        page.close()
    finally:
        pw.stop()


if __name__ == "__main__":
    main()
