---
name: notebooklm
description: "Manage Google NotebookLM notebooks: create, list, find, read, delete, add sources. Requires Chrome with remote debugging (CHROME_REMOTE_URL)."
command: ["python3", "notebooklm.py"]
args: ["--action", "{{action}}", "--name", "{{name}}", "--query", "{{query}}", "--notebook-id", "{{notebook_id}}", "--source", "{{source}}", "--confirm", "{{confirm}}"]
timeout: 120
schema:
  action:
    type: string
    description: "Action: list, create, find, read, delete, add_source"
    required: true
  name:
    type: string
    description: "Notebook name (for create)"
    required: false
  query:
    type: string
    description: "Search query (for find)"
    required: false
  notebook_id:
    type: string
    description: "Notebook ID or full URL (for read, delete, add_source)"
    required: false
  source:
    type: string
    description: "File path or URL to add as source (for add_source)"
    required: false
  confirm:
    type: string
    description: "Set to 'yes' to confirm destructive actions like delete"
    required: false
---

# NotebookLM

Browser automation skill for Google NotebookLM. Connects to your logged-in Chrome instance via Chrome DevTools Protocol.

## Setup

1. Launch Chrome with remote debugging:
   ```bash
   # macOS
   /Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222
   ```

2. Log into your Google account in that Chrome window.

3. Set the environment variable:
   ```bash
   export CHROME_REMOTE_URL="ws://127.0.0.1:9222"
   ```

4. Install Python dependency:
   ```bash
   pip install playwright && playwright install chromium
   ```

## Actions

| Action | Required Params | Description |
|--------|----------------|-------------|
| `list` | — | List all notebooks |
| `create` | `name` | Create a new notebook |
| `find` | `query` | Search notebooks by name |
| `read` | `notebook_id` | Read notebook details and sources |
| `delete` | `notebook_id`, `confirm=yes` | Delete a notebook (requires confirmation) |
| `add_source` | `notebook_id`, `source` | Add a PDF/URL/text source to a notebook |
