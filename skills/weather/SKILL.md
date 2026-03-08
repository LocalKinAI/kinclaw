---
name: weather
description: Get current weather for a location using wttr.in
command: [curl, "-s", "https://wttr.in/{{location}}?format=3"]
schema:
  location:
    type: string
    description: City name or location (e.g. Tokyo, London, "New York")
    required: true
timeout: 10
---
# Weather

Fetches current weather from wttr.in. No API key needed.
