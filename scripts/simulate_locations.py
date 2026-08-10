#!/usr/bin/env python3
"""Simulate moving GPS for an RRT unit or an incident.

Pushes coordinates along a straight line every N seconds so you can test
WebSocket live updates without leaving the apartment.

Usage:
    python3 simulate_locations.py rrt <unit-id>
    python3 simulate_locations.py incident <incident-id>

    BASE_URL=http://127.0.0.1:8080/api/v1 STEP_SECONDS=1.5 python3 ...
"""
import json
import os
import sys
import time
import urllib.request

BASE_URL = os.environ.get("BASE_URL", "http://127.0.0.1:8080/api/v1")
STEP_SECONDS = float(os.environ.get("STEP_SECONDS", "1.5"))
STEPS = int(os.environ.get("STEPS", "20"))

START = (12.9236, 100.8824)  # Walking Street, Pattaya
END = (12.9336, 100.8774)    # a bit north-west


def push(url: str, lat: float, lng: float) -> None:
    body = json.dumps({"lat": lat, "lng": lng}).encode()
    req = urllib.request.Request(
        url, data=body, headers={"Content-Type": "application/json"}, method="PUT"
    )
    with urllib.request.urlopen(req) as resp:
        print(f"lat={lat:.5f} lng={lng:.5f} -> {resp.read().decode()}")


def main() -> None:
    if len(sys.argv) != 3:
        print(__doc__)
        sys.exit(1)

    kind, entity_id = sys.argv[1], sys.argv[2]
    url = f"{BASE_URL}/{kind}s/{entity_id}/location"

    for i in range(STEPS):
        t = i / max(STEPS - 1, 1)
        lat = START[0] + (END[0] - START[0]) * t
        lng = START[1] + (END[1] - START[1]) * t
        try:
            push(url, lat, lng)
        except Exception as err:
            print(f"error: {err}")
            sys.exit(1)
        time.sleep(STEP_SECONDS)


if __name__ == "__main__":
    main()
