#!/usr/bin/env python3
"""Fail if any benchmark regressed beyond the threshold.

Reads benchstat CSV comparison (old vs new) from stdin. benchstat only
reports a numeric delta for statistically significant changes (p < 0.05);
noisy comparisons are marked "~" and ignored.
"""
import csv
import sys

THRESHOLD = 20.0  # percent slower

def delta_pct(s):
    s = s.strip()
    if not s or s == "~":
        return None
    sign = 1.0
    if s.startswith("+"):
        s = s[1:]
    elif s.startswith("-"):
        sign = -1.0
        s = s[1:]
    s = s.rstrip("%")
    if s in ("∞", "Inf", "inf", "+Inf"):
        return sign * float("inf")
    try:
        return sign * float(s)
    except ValueError:
        return None

def main():
    regressions = []
    for row in csv.reader(sys.stdin):
        if len(row) < 7 or row[0] in ("", "geomean"):
            continue
        d = delta_pct(row[5])  # vs base column
        if d is not None and d > THRESHOLD:
            regressions.append((row[0], d))

    if not regressions:
        print("no regressions beyond +%.0f%%" % THRESHOLD)
        return 0

    print("performance regressions beyond +%.0f%%:" % THRESHOLD)
    for name, d in sorted(regressions):
        if d == float("inf"):
            print("  %s: +∞ (new cost where none before)" % name)
        else:
            print("  %s: +%.1f%%" % (name, d))
    return 1


if __name__ == "__main__":
    sys.exit(main())
