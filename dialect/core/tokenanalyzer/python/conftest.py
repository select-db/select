"""
Make every diagnostic the suite produces prove it can be drawn.

A diagnostic an editor cannot draw reads to a user exactly like no diagnostic,
and a test that counts rule ids cannot tell the two apart. In production
analyze widens such a range so something is visible; here it raises, so a rule
that cannot point at its own clause fails a test instead of shipping.

Every rule test module goes through analyze, so this covers every rule and
covers a rule added later without anyone adding it to a list.
"""
import os

os.environ["SELECT_STRICT_DIAGNOSTICS"] = "1"
