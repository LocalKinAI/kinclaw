---
name: weather
description: |
  Current conditions plus a three-day forecast (today / tomorrow / day
  after) for a place, in metric, from wttr.in. No API key. Use this for
  any weather question — including "明天" and "后天" — before reaching
  for web search.
command: [python3, "weather.py", "{{location}}"]   # exec-form skills run with cwd = the skill dir
schema:
  location:
    type: string
    description: City name or location (e.g. Tokyo, London, 上海, "New York")
    required: true
timeout: 15
---
# Weather

Fetches wttr.in's JSON forecast and prints four lines: now, today,
tomorrow, the day after — condition, temperature range in °C, and the
day's peak chance of rain. Compact enough to read out loud.
