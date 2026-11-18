#!/usr/bin/env python3
"""Self-test for the notice-date arithmetic in check-outsourcing.py."""
import datetime as dt, importlib.util, os
spec = importlib.util.spec_from_file_location("co", os.path.join(os.path.dirname(os.path.abspath(__file__)), "check-outsourcing.py"))
co = importlib.util.module_from_spec(spec); spec.loader.exec_module(co)
assert co.easter(2027) == dt.date(2027, 3, 28), co.easter(2027)
# Start Monday 3 May 2027: counting 20 working days back skips weekends and Freedom Day (Tuesday 27 April), so the
# notice is due by Friday 2 April 2027.
d = co.working_days_before(dt.date(2027, 5, 3), 20)
assert d == dt.date(2027, 4, 2), d
assert co.working_days_before(dt.date(2027, 1, 4), 1) == dt.date(2026, 12, 31)
print("ok   outsourcing notice dates")
