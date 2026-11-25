#!/usr/bin/env python3
"""The Chromium bump tools read every platform's #if, not the Mac's alone: a delegate method
only Android or Linux has is found, so a new one is reported on the next bump. Chromium 158's
Android-only ContentBrowserClient::IsContentSchemeURLNavigationAllowed went unseen, and Android
opened a downloaded file as a blank tab (docs/chromium-upgrade.md).

    python3 tools/platform_methods_test.py
"""

import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.realpath(__file__)))
import gen_hooks  # noqa: E402
import web_features_bump  # noqa: E402

HEADER = """
#ifndef CONTENT_PUBLIC_BROWSER_FAKE_CLIENT_H_
#define CONTENT_PUBLIC_BROWSER_FAKE_CLIENT_H_
class FakeClient {
 public:
  virtual void Everywhere();
#if BUILDFLAG(IS_ANDROID)
  virtual bool PhoneOnly(const GURL& url);
#else
  virtual void NotOnPhones();
#endif
#if BUILDFLAG(IS_LINUX) || BUILDFLAG(IS_CHROMEOS)
  virtual void LinuxOnly(
      int child_process_id);
#endif
#if BUILDFLAG(IS_POSIX) && !BUILDFLAG(IS_MAC)
  virtual void PhonesAndLinux();
#endif
#if BUILDFLAG(IS_MAC)
  virtual void MacOnly();
#elif BUILDFLAG(IS_WIN)
  virtual void WindowsOnly();
#endif
};
#endif
"""


class PlatformMethods(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.dir.cleanup)
        with open(os.path.join(self.dir.name, "fake_client.h"), "w") as f:
            f.write(HEADER)

    def test_each_method_has_the_platforms_its_if_lets_it_in(self):
        self.assertEqual(web_features_bump.methods(self.dir.name, "fake_client.h"), {
            "Everywhere": ["mac", "android", "linux"],
            "PhoneOnly": ["android"],
            "NotOnPhones": ["mac", "linux"],
            "LinuxOnly": ["linux"],
            "PhonesAndLinux": ["android", "linux"],
            "MacOnly": ["mac"],
            "WindowsOnly": [],
        })

    def test_gen_hooks_lists_what_the_macs_tree_lacks(self):
        # clang, on the Mac's tree, sees the Mac's methods; the rest are listed by name.
        seen = {"Everywhere", "NotOnPhones", "MacOnly"}
        self.assertEqual(gen_hooks.elsewhere_only(self.dir.name, "fake_client.h", seen), {
            "LinuxOnly": ["linux"],
            "PhoneOnly": ["android"],
            "PhonesAndLinux": ["android", "linux"],
        })

    @unittest.skipUnless(os.path.exists(os.path.join(gen_hooks.SRC, "content/public/browser/content_browser_client.h")),
                         "no Chromium tree")
    def test_the_one_158_brought_is_found_in_the_real_tree(self):
        found = web_features_bump.methods(gen_hooks.SRC, "content/public/browser/content_browser_client.h")
        self.assertEqual(found.get("IsContentSchemeURLNavigationAllowed"), ["android"])


if __name__ == "__main__":
    unittest.main()
