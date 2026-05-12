---
name: location
description: |
  Get the user's current GPS location. Auto-detects the best backend
  for the host OS:

    macOS  →  corelocationcli  (CoreLocation — cell / WiFi / GPS)
    Linux  →  gdbus + geoclue2  (kernel-driver / WiFi / Mozilla MLS)
    fallback → IP geolocation   (ipapi.co — city-level only)

  Returns coordinates / address / city / full details — request what
  you need with the `format` arg.

  When to use this vs the {{location}} system prompt context:
    - {{location}} is set from $KINCLAW_LOCATION env var — "where the
      user generally is" (home / office). Static, free, always available.
    - This skill reads the OS location service in real time — "where the
      user is right NOW". Use when user might be traveling / driving /
      out, or when meter-level precision matters.

  First invocation prompts for permission (macOS Location Services /
  Linux Geoclue per-app authorization through xdg-desktop-portal).
  Grant it for whichever process kinclaw runs in.

  First-time setup:
    macOS:  brew install corelocationcli
    Linux:  apt install geoclue-2.0 jq curl    # or pacman -S / dnf install
            (gdbus ships with glib2 — already present on every desktop Linux)
command:
  - sh
  - -c
  - |
    FORMAT="$1"
    [ "$FORMAT" = "" ] && FORMAT="coords"

    # ──── Backend detection ────────────────────────────────────────
    # Prefer corelocationcli on macOS (CoreLocation is the most accurate
    # and lowest-latency option there). Linux uses geoclue2 via gdbus
    # (universal — ships with glib2). IP fallback is last resort for
    # both, useful for servers / CI / Pi without GPS hardware.

    OS=$(uname -s)
    BACKEND=""
    if [ "$OS" = "Darwin" ] && command -v corelocationcli >/dev/null 2>&1; then
      BACKEND="corelocationcli"
    elif [ "$OS" = "Linux" ] && command -v gdbus >/dev/null 2>&1; then
      BACKEND="geoclue"
    elif command -v curl >/dev/null 2>&1; then
      BACKEND="ip"
    else
      echo "no location backend available" >&2
      case "$OS" in
        Darwin) echo "  install: brew install corelocationcli" >&2 ;;
        Linux)  echo "  install: apt install geoclue-2.0 jq curl" >&2 ;;
      esac
      exit 1
    fi

    # ──── Backend: corelocationcli (macOS) ─────────────────────────
    if [ "$BACKEND" = "corelocationcli" ]; then
      case "$FORMAT" in
        coords)
          LL=$(corelocationcli -once -format "%latitude,%longitude")
          printf 'GPS coordinates (lat,lon): %s\n' "$LL"
          ;;
        address)
          ADDR=$(corelocationcli -once -format "%address")
          printf 'GPS address: %s\n' "$ADDR"
          ;;
        city)
          CITY=$(corelocationcli -once -format "%locality")
          printf 'GPS city: %s\n' "$CITY"
          ;;
        full)
          printf 'GPS (full reading from CoreLocation):\n'
          corelocationcli -once
          ;;
        *)
          echo "unknown format: $FORMAT (expected: coords | address | city | full)" >&2
          exit 1
          ;;
      esac
      exit 0
    fi

    # ──── Backend: geoclue2 via gdbus (Linux) ──────────────────────
    # Geoclue2 D-Bus dance:
    #   1) Manager.GetClient → client object path
    #   2) Set DesktopId + RequestedAccuracyLevel on the client
    #   3) Client.Start → triggers actual fix
    #   4) Read Location property → returns location object path
    #   5) Read Latitude / Longitude / Description from location object
    #   6) Client.Stop → release the resource
    if [ "$BACKEND" = "geoclue" ]; then
      gdbus_call() {
        gdbus call --system --dest org.freedesktop.GeoClue2 "$@" 2>/dev/null
      }

      # 1) Create a Client
      CLIENT_OUT=$(gdbus_call --object-path /org/freedesktop/GeoClue2/Manager \
                              --method org.freedesktop.GeoClue2.Manager.GetClient)
      if [ -z "$CLIENT_OUT" ]; then
        echo "Geoclue2 not running. Start with: systemctl --user start geoclue.service" >&2
        echo "Falling back to IP geolocation..." >&2
        BACKEND="ip"
      else
        CLIENT_PATH=$(printf '%s' "$CLIENT_OUT" | sed -n "s|.*'\\(/org/freedesktop/GeoClue2/Client/[^']*\\)'.*|\\1|p")
        if [ -z "$CLIENT_PATH" ]; then
          echo "Geoclue2: couldn't parse client path from: $CLIENT_OUT" >&2
          BACKEND="ip"
        fi
      fi

      if [ "$BACKEND" = "geoclue" ]; then
        # 2) Set DesktopId (required for newer geoclue) + accuracy 8 = exact
        gdbus_call --object-path "$CLIENT_PATH" \
          --method org.freedesktop.DBus.Properties.Set \
          org.freedesktop.GeoClue2.Client DesktopId '<"kinclaw">' >/dev/null

        gdbus_call --object-path "$CLIENT_PATH" \
          --method org.freedesktop.DBus.Properties.Set \
          org.freedesktop.GeoClue2.Client RequestedAccuracyLevel '<uint32 8>' >/dev/null

        # 3) Start
        gdbus_call --object-path "$CLIENT_PATH" \
          --method org.freedesktop.GeoClue2.Client.Start >/dev/null

        # 4) Poll for Location property (up to ~10s)
        LOC_PATH=""
        for i in 1 2 3 4 5 6 7 8 9 10; do
          LP_OUT=$(gdbus_call --object-path "$CLIENT_PATH" \
            --method org.freedesktop.DBus.Properties.Get \
            org.freedesktop.GeoClue2.Client Location)
          LOC_PATH=$(printf '%s' "$LP_OUT" | sed -n "s|.*'\\(/org/freedesktop/GeoClue2/Client/[^']*Location[^']*\\)'.*|\\1|p")
          [ -n "$LOC_PATH" ] && [ "$LOC_PATH" != "/" ] && break
          sleep 1
        done

        if [ -z "$LOC_PATH" ] || [ "$LOC_PATH" = "/" ]; then
          # 6) Cleanup
          gdbus_call --object-path "$CLIENT_PATH" \
            --method org.freedesktop.GeoClue2.Client.Stop >/dev/null
          echo "Geoclue2 didn't return a fix within 10s — agent may need to grant permission via xdg-desktop-portal dialog." >&2
          echo "Falling back to IP geolocation..." >&2
          BACKEND="ip"
        else
          # 5) Read Latitude / Longitude / Description
          read_prop() {
            local prop="$1"
            local out
            out=$(gdbus_call --object-path "$LOC_PATH" \
              --method org.freedesktop.DBus.Properties.Get \
              org.freedesktop.GeoClue2.Location "$prop")
            # Strip the variant wrapper: "(<value>,)" → "value"
            printf '%s' "$out" | sed -E "s/^\\(<(.*)>,?\\)\$/\\1/" | sed -E "s/^'(.*)'\$/\\1/"
          }
          LAT=$(read_prop Latitude)
          LON=$(read_prop Longitude)
          ALT=$(read_prop Altitude)
          ACC=$(read_prop Accuracy)
          DESCR=$(read_prop Description)

          # Cleanup
          gdbus_call --object-path "$CLIENT_PATH" \
            --method org.freedesktop.GeoClue2.Client.Stop >/dev/null

          case "$FORMAT" in
            coords)
              printf 'GPS coordinates (lat,lon): %s,%s\n' "$LAT" "$LON"
              ;;
            address|city)
              # Geoclue doesn't reverse-geocode; query Nominatim (OpenStreetMap).
              if command -v curl >/dev/null 2>&1; then
                NOM=$(curl -sS --max-time 5 \
                  -H 'User-Agent: KinClaw/1.0 (https://localkin.dev)' \
                  "https://nominatim.openstreetmap.org/reverse?lat=$LAT&lon=$LON&format=json")
                if [ "$FORMAT" = "address" ]; then
                  ADDR=$(printf '%s' "$NOM" | sed -nE 's/.*"display_name":"([^"]+)".*/\1/p')
                  printf 'GPS address: %s\n' "${ADDR:-(reverse geocode failed)}"
                else
                  CITY=$(printf '%s' "$NOM" | sed -nE 's/.*"city":"([^"]+)".*/\1/p')
                  [ -z "$CITY" ] && CITY=$(printf '%s' "$NOM" | sed -nE 's/.*"town":"([^"]+)".*/\1/p')
                  [ -z "$CITY" ] && CITY=$(printf '%s' "$NOM" | sed -nE 's/.*"village":"([^"]+)".*/\1/p')
                  printf 'GPS city: %s\n' "${CITY:-(reverse geocode failed)}"
                fi
              else
                echo "curl not installed — can't reverse-geocode. Coords: $LAT,$LON" >&2
                exit 1
              fi
              ;;
            full)
              printf 'GPS (Geoclue2):\n'
              printf '  latitude:    %s\n' "$LAT"
              printf '  longitude:   %s\n' "$LON"
              printf '  altitude:    %s\n' "$ALT"
              printf '  accuracy_m:  %s\n' "$ACC"
              printf '  description: %s\n' "$DESCR"
              ;;
            *)
              echo "unknown format: $FORMAT (expected: coords | address | city | full)" >&2
              exit 1
              ;;
          esac
          exit 0
        fi
      fi
    fi

    # ──── Backend: IP geolocation (last resort) ────────────────────
    # Useful for servers / CI / Pi without GPS. Accuracy: city-level only.
    if [ "$BACKEND" = "ip" ]; then
      JSON=$(curl -sS --max-time 5 https://ipapi.co/json)
      if [ -z "$JSON" ]; then
        echo "IP geolocation failed (no network?)" >&2
        exit 1
      fi
      LAT=$(printf '%s' "$JSON" | sed -nE 's/.*"latitude": *([^,]+).*/\1/p')
      LON=$(printf '%s' "$JSON" | sed -nE 's/.*"longitude": *([^,]+).*/\1/p')
      CITY=$(printf '%s' "$JSON" | sed -nE 's/.*"city": *"([^"]+)".*/\1/p')
      REGION=$(printf '%s' "$JSON" | sed -nE 's/.*"region": *"([^"]+)".*/\1/p')
      COUNTRY=$(printf '%s' "$JSON" | sed -nE 's/.*"country_name": *"([^"]+)".*/\1/p')

      case "$FORMAT" in
        coords)
          printf 'GPS coordinates (lat,lon, via IP geolocation — city-level accuracy): %s,%s\n' "$LAT" "$LON"
          ;;
        address)
          printf 'GPS address (via IP — city-level only): %s, %s, %s\n' "$CITY" "$REGION" "$COUNTRY"
          ;;
        city)
          printf 'GPS city (via IP): %s\n' "$CITY"
          ;;
        full)
          printf 'GPS (full reading via IP geolocation — city-level accuracy):\n'
          printf '  latitude:   %s\n' "$LAT"
          printf '  longitude:  %s\n' "$LON"
          printf '  city:       %s\n' "$CITY"
          printf '  region:     %s\n' "$REGION"
          printf '  country:    %s\n' "$COUNTRY"
          ;;
        *)
          echo "unknown format: $FORMAT (expected: coords | address | city | full)" >&2
          exit 1
          ;;
      esac
      exit 0
    fi
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
        full     - all available fields (lat / lon / altitude / accuracy / address / time)
timeout: 20
---

# location — real-time GPS, cross-platform

Auto-detected backend per OS:

| OS | Backend | Accuracy | Permission |
|---|---|---|---|
| macOS | `corelocationcli` (CoreLocation) | cell / WiFi / GPS (meter-level) | System Settings → Privacy & Security → Location |
| Linux | `gdbus` + Geoclue2 | WiFi / Mozilla MLS / kernel-DRM | xdg-desktop-portal dialog (per-app) |
| any | `curl ipapi.co` fallback | city-level only | none (uses your IP) |

Two-tier location story for KinClaw:

| Layer | Where | When to use |
|---|---|---|
| `{{location}}` substitution | Soul prompt, from `$KINCLAW_LOCATION` env | Persistent context — "user lives in Beijing" |
| `location` skill (this) | Tool call to OS location service | Real-time GPS — "user is at $current_lat,$lon" |

The skill is opt-in: not installed → clear error message + install hint.
Already installed → uses the OS's actual location services.

## Privacy

- **macOS**: triggers Apple's standard Location Services permission flow.
  macOS asks ONCE per process; subsequent calls reuse the grant. Revoke
  via System Settings → Privacy & Security → Location Services.
- **Linux**: Geoclue2 uses `xdg-desktop-portal` for permission. On GNOME /
  KDE it prompts via the portal dialog; on Sway/i3 the dialog may not
  appear (in which case grant via `gnome-control-center privacy` once).
- **IP fallback**: no permission needed because no GPS hardware is queried —
  ipapi.co geolocates your public IP. City-level only.

## Usage

```
location                       → "37.7749,-122.4194"
location format=address        → "1 Apple Park Way, Cupertino, CA"
location format=city           → "Cupertino"
location format=full           → multi-line: lat/lon/alt/accuracy/...
```

The backend is auto-selected at invocation time; the agent doesn't need
to know whether it's running on macOS or Linux.

## Linux setup (one-time)

```bash
# Debian / Ubuntu
sudo apt install geoclue-2.0 curl

# Fedora
sudo dnf install geoclue2 curl

# Arch
sudo pacman -S geoclue curl

# Verify geoclue is running
systemctl --user status geoclue.service  # may be system-wide, try without --user too
```

If Geoclue2 doesn't return a fix within 10 seconds (often the case on
servers / Pi without WiFi positioning), the skill auto-falls-back to IP
geolocation. Forge a derivative skill if you want different fallback
behavior (e.g. cache last good fix, exponential retry).
