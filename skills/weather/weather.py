#!/usr/bin/env python3
"""Three-day forecast from wttr.in in a form a voice can read out.

The old skill was one line of `format=3` — current conditions only, in
Fahrenheit for anyone whose IP sits in the US. Ask "明天上海天气" and
the model had nothing for tomorrow, went looking on the web, and looped.
This asks for the JSON forecast, reads what matters, and prints it in
metric: now, today, tomorrow, the day after.
"""
import json
import sys
import urllib.parse
import urllib.request

loc = sys.argv[1].strip() if len(sys.argv) > 1 else ""
url = "https://wttr.in/%s?format=j1" % urllib.parse.quote(loc)
try:
    req = urllib.request.Request(url, headers={"User-Agent": "curl/8.0"})
    with urllib.request.urlopen(req, timeout=12) as r:
        d = json.load(r)
except Exception as e:  # noqa: BLE001 — anything here means "no forecast"
    print("weather unavailable for %r: %s" % (loc, e), file=sys.stderr)
    sys.exit(1)


def desc(x):
    return (x.get("weatherDesc") or [{"value": "?"}])[0]["value"].strip()


cc = d["current_condition"][0]
area = (d.get("nearest_area") or [{}])[0]
name = loc or (area.get("areaName") or [{"value": ""}])[0]["value"]
print("%s 现在: %s, %s°C (体感 %s°C), 湿度 %s%%, 风 %s km/h" % (
    name, desc(cc), cc["temp_C"], cc["FeelsLikeC"], cc["humidity"], cc["windspeedKmph"]))

# Dates are the place's own calendar — asking about Shanghai from New
# York at night, "today" there is already tomorrow here. The date is
# printed next to the label so the model can say which it means.
labels = ["今天", "明天", "后天"]
for i, day in enumerate(d.get("weather", [])[:3]):
    hours = day.get("hourly") or []
    # Eight 3-hour slots a day; index 4 is noon, the one people mean
    # when they ask what a day will be like.
    noon = hours[4] if len(hours) > 4 else (hours[0] if hours else {})
    rain = max((int(h.get("chanceofrain", 0) or 0) for h in hours), default=0)
    print("%s (当地 %s): %s, %s–%s°C, 降雨概率 %d%%" % (
        labels[i], day["date"][5:], desc(noon), day["mintempC"], day["maxtempC"], rain))
