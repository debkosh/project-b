# Upgrading Chromium

What to run when a new version of content is taken, besides the build itself. Each step's own
doc has the detail; this is the list, in order.

1. `python3 tools/web_features_bump.py`: what the web-features registry lacks and what it names
   that Chromium dropped ([web-features.md](web-features.md), "After a Chromium bump"). It
   compares with the hooks table, so it goes before the next step, and it needs no build.
2. `python3 tools/gen_hooks.py` against the fresh `out/pb`: content's hooks as Go knows them
   ([content-hooks.md](content-hooks.md), "When Chromium is upgraded"). Check the result in and
   rebuild.
3. `tools/form-matching.sh update`: DuckDuckGo's latest field matching, which finds a page's
   sign-in fields ([password-autofill.md](password-autofill.md), "Detection"). It is checked
   in a real Chromium, so it is measured again on the new one: the script prints the scores
   before and after. Then `tools/test.sh mock`, and commit the three files it names.
