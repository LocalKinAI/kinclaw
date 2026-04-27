---
name: app_open_clean
description: |
  Open a macOS app AND dismiss any blocking welcome / "What's New" /
  setup-prompt sheet that appeared on launch. Returns the app name
  and whether a modal was dismissed. Use this INSTEAD of `shell open
  -a X` when first opening an app you haven't driven before — saves
  the agent from the common failure mode of typing into a welcome
  modal instead of the actual UI.

  Looks for these dismiss buttons in priority order across any sheet
  or window of the frontmost app:
    Continue · Get Started · Skip · Later · Not Now · Got It ·
    Maybe Later · Done · Cancel

  Generic — works for first-launch Apple apps (Reminders, Mail,
  Calendar, Photos, Maps, Music) and third-party apps that follow
  the same modal pattern.
command:
  - sh
  - -c
  - |
    APP="$1"
    [ -z "$APP" ] && { echo "app name required" >&2; exit 1; }

    open -a "$APP"
    # Most welcome sheets appear within 1.5s of launch. Sleep more on
    # slow boots — better to be late than to dismiss-before-render.
    sleep 1.5

    # Walk the frontmost app's sheets + windows; click the first
    # matching dismiss button by priority. Errors silenced — if the
    # app has no modal, we just exit with no-op success.
    DISMISSED=$(osascript 2>/dev/null <<'APPLESCRIPT'
    set dismissBtns to {"Continue", "Get Started", "Skip", "Later", "Not Now", "Got It", "Maybe Later", "Done", "Cancel"}
    set didClick to ""
    tell application "System Events"
      try
        set frontProc to first application process whose frontmost is true
        tell frontProc
          repeat with w in windows
            -- Sheets (attached modal dialogs)
            try
              repeat with s in sheets of w
                repeat with btnName in dismissBtns
                  if exists (button btnName of s) then
                    click button btnName of s
                    set didClick to btnName as string
                    exit repeat
                  end if
                end repeat
                if didClick is not "" then exit repeat
              end repeat
            end try
            if didClick is not "" then exit repeat
            -- Standalone window that looks like welcome (has Continue/Get Started)
            try
              repeat with btnName in {"Continue", "Get Started", "Got It"}
                if exists (button btnName of w) then
                  click button btnName of w
                  set didClick to btnName as string
                  exit repeat
                end if
              end repeat
            end try
            if didClick is not "" then exit repeat
          end repeat
        end tell
      end try
    end tell
    return didClick
    APPLESCRIPT
    )

    # Some welcome flows are multi-step. Run a second pass to catch
    # the next sheet (e.g. Reminders has "What's New" → may auto-open
    # iCloud sync prompt). Quick second pass, no sleep needed.
    if [ -n "$DISMISSED" ]; then
      sleep 0.4
      DISMISSED2=$(osascript 2>/dev/null <<'APPLESCRIPT'
    set dismissBtns to {"Continue", "Get Started", "Skip", "Later", "Not Now", "Got It", "Maybe Later", "Done", "Cancel"}
    set didClick to ""
    tell application "System Events"
      try
        set frontProc to first application process whose frontmost is true
        tell frontProc
          repeat with w in windows
            try
              repeat with s in sheets of w
                repeat with btnName in dismissBtns
                  if exists (button btnName of s) then
                    click button btnName of s
                    set didClick to btnName as string
                    exit repeat
                  end if
                end repeat
                if didClick is not "" then exit repeat
              end repeat
            end try
            if didClick is not "" then exit repeat
          end repeat
        end tell
      end try
    end tell
    return didClick
    APPLESCRIPT
    )
    fi

    if [ -n "$DISMISSED" ]; then
      printf 'opened: %s\ndismissed: %s' "$APP" "$DISMISSED"
      [ -n "$DISMISSED2" ] && printf ' → %s' "$DISMISSED2"
      printf '\n'
    else
      printf 'opened: %s\ndismissed: (no modal found)\n' "$APP"
    fi
  - "_"
args:
  - "{{app}}"
schema:
  app:
    type: string
    description: App name (e.g. "Reminders", "Mail", "Calendar") or bundle ID. Whatever `open -a` accepts.
    required: true
timeout: 15
---

# app_open_clean — open + dismiss welcome modal

A common failure pattern when an LLM agent first drives a Mac app:
launch the app → start typing or clicking → realize too late that a
"What's New" or "Welcome" sheet was sitting on top of the real UI
the whole time. The keystrokes go nowhere or hit the wrong target.
The app looks unresponsive but is actually working fine; the modal
is invisible to the AX-walk because it's a separate sheet.

This skill handles both phases in a single tool call:

1. `open -a <app>` — launches via Launch Services
2. `osascript` walks the frontmost process's windows + sheets, clicks
   the highest-priority dismiss button it finds, repeats once for
   stacked welcome flows.

If the app has no modal (already dismissed, or never had one), the
script exits clean and reports "no modal found" — completely safe
to call on already-open apps.

## When to use

- You're driving a built-in macOS app you haven't touched in this
  session: Reminders, Mail, Calendar, Photos, Maps, Music, etc.
- You just upgraded macOS — every app's "What's New" fires once.
- A third-party app you've never opened on this machine (Bear,
  Slack, Notion, etc.).

## When NOT to use

- App is already running and you've already used it — `shell open -a`
  is enough, no modal will appear.
- The "modal" you want to keep — e.g. you opened Mail to use the Add
  Account flow on purpose. (Use `shell open -a Mail` directly then.)

## Examples

```
app_open_clean app=Reminders
  → opened: Reminders
    dismissed: Continue → Got It

app_open_clean app=Calculator
  → opened: Calculator
    dismissed: (no modal found)
```

## Why a SKILL.md and not native

Pure shell + osascript. No Go state, no AX walking via kinax that
this skill doesn't already get from the standard `ui` tool. Sticks
to the "thin kernel + fat skill" thesis — apps come and go, modal
shapes evolve, this is exactly what `forge` should be writing.

## Override / extend

If your installed app has a different dismiss button label not in
the priority list (e.g. "Begin", "OK, got it", "下一步"), edit this
SKILL.md and add it to `dismissBtns`. Or `forge` a sibling skill
named after the specific app.
