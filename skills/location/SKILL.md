---
name: location
description: |
  Get the user's current GPS location via macOS CoreLocation
  (`corelocationcli`). Returns coordinates / address / city / full
  details — request what you need with the `format` arg.

  When to use this vs the {{location}} system prompt context:
    - {{location}} is set from $KINCLAW_LOCATION env var — "where the
      user generally is" (home / office). Static, free, always available.
    - This skill reads CoreLocation in real time — "where the user is
      right NOW". Use when user might be traveling / driving / out, or
      when meter-level precision matters (env var is usually city-level).

  First invocation triggers a macOS Location Services permission prompt.
  Grant it for whichever process kinclaw runs in (Terminal, iTerm, etc.).

  First-time setup:
    brew install corelocationcli
command:
  - sh
  - -c
  - |
    if ! command -v corelocationcli >/dev/null; then
      echo "corelocationcli not installed. Setup:" >&2
      echo "  brew install corelocationcli" >&2
      exit 1
    fi
    FORMAT="$1"
    [ "$FORMAT" = "" ] && FORMAT="coords"
    # Wrap each output in a labelled "GPS" preamble. Bare lat/lon
    # numbers tend to confuse smaller models (observed live with
    # Kimi K2.5: it received "37.398 -122.063" as a tool result and
    # then returned an empty assistant message — model didn't realize
    # that's the answer to interpret + relay). Adding a text frame
    # ("GPS coordinates: ...") makes the tool result self-describing.
    case "$FORMAT" in
      coords)
        # Default: lat,lon — easy to parse, easy to feed downstream.
        LL=$(corelocationcli -once -format "%latitude,%longitude")
        printf 'GPS coordinates (lat,lon): %s\n' "$LL"
        ;;
      address)
        # Reverse-geocoded street address (where Apple's geocoder
        # supports the user's locale).
        ADDR=$(corelocationcli -once -format "%address")
        printf 'GPS address: %s\n' "$ADDR"
        ;;
      city)
        # Just the locality / city name.
        CITY=$(corelocationcli -once -format "%locality")
        printf 'GPS city: %s\n' "$CITY"
        ;;
      full)
        # Everything CoreLocation knows: lat/lon/alt/speed/direction/
        # accuracy/time/address — multi-line. Useful when the agent
        # needs to reason about altitude (hiking?), speed (driving?),
        # or accuracy (cell vs WiFi vs GPS).
        printf 'GPS (full reading from CoreLocation):\n'
        corelocationcli -once
        ;;
      *)
        echo "unknown format: $FORMAT (expected: coords | address | city | full)" >&2
        exit 1
        ;;
    esac
  - "_"
args:
  - "{{format}}"
schema:
  format:
    type: string
    description: |
      Output shape:
        coords   - "37.7749,-122.4194" (default; lat,lon)
        address  - reverse-geocoded street address
        city     - locality name only
        full     - all CoreLocation data (lat / lon / altitude / speed / accuracy / address / time)
timeout: 15
---

# location — real-time GPS via macOS CoreLocation

Two-tier location story for KinClaw:

| Layer | Where | When to use |
|---|---|---|
| `{{location}}` substitution | Soul prompt, from `$KINCLAW_LOCATION` env | Persistent context — "user lives in Beijing" |
| `location` skill (this) | Tool call to `corelocationcli` | Real-time GPS — "user is at $current_lat,$lon" |

The skill is opt-in: not installed → clear error message + install hint.
Already installed → uses macOS's actual location services (cell / WiFi /
GPS, in that order of accuracy depending on context).

## Privacy

`corelocationcli` triggers Apple's standard Location Services permission
flow. macOS asks ONCE per process; subsequent calls reuse the grant. To
revoke later: System Settings → Privacy & Security → Location Services →
toggle off the granting app (Terminal / iTerm / kinclaw).

## Usage

```
location                       → "37.7749,-122.4194"
location format=address        → "1 Apple Park Way, Cupertino, CA"
location format=city           → "Cupertino"
location format=full           → multi-line: lat/lon/alt/speed/accuracy/...
```

## Why a SKILL.md and not native

corelocationcli is a single-purpose CLI; wrapping it as native Go would
require either embedding CoreLocation via cgo (breaks zero-cgo) or
shelling out anyway. Pure shell wrapper, no kernel state, perfect
SKILL.md territory.

If you want fancier behavior (e.g. cache last reading, fail-soft when
permission denied, geofencing), forge a derivative skill — this one is
intentionally minimal.
