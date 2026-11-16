"""The page Links to other sites is checked on (docs/site-features.md#checking-it): each of the
ten ways a page leaves for another site, shared by the Mac's driver and Android's.

The page is served at /leave on its own host, the other site on another host of the same
server: two names for this machine, which are different sites. Each press counts in the page
(#count), and 1.5 s after it the page asks its own host for /alive?<n>: a page that is still
there asks, one that left or reloaded doesn't. /leave-redirect is the same-site link the
server sends on to the other site.
"""

# Each way: its number, its words on the page, whether it opens a tab, and the paths the other
# site is asked for when it goes.
WAYS = [
    (1, "1 link", False, ["/a1"]),
    (2, "2 target=_blank", True, ["/a2"]),
    (3, "3 noopener", True, ["/a3"]),
    (4, "4 window.open", True, ["/b4"]),
    (5, "5 location.href", False, ["/b5"]),
    (6, "6 location.replace", False, ["/b6"]),
    (7, "7 form", False, ["/f7"]),
    (8, "8 popup, then its location", True, ["/b8"]),
    (9, "9 same-site redirect", False, ["/r9"]),
    (10, "10 popup and leave", True, ["/ad1", "/ad2"]),
]


def page(other):
    """The page, with other the other site's origin, as http://host:port."""
    o = other
    items = [
        f'<a id=l1 href="{o}/a1">1 link</a>',
        f'<a id=l2 href="{o}/a2" target="_blank">2 target=_blank</a>',
        f'<a id=l3 href="{o}/a3" target="_blank" rel="noopener">3 noopener</a>',
        f"<button id=l4 onclick=\"window.open('{o}/b4')\">4 window.open</button>",
        f"<button id=l5 onclick=\"location.href='{o}/b5'\">5 location.href</button>",
        f"<button id=l6 onclick=\"location.replace('{o}/b6')\">6 location.replace</button>",
        f'<form action="{o}/f7"><button id=l7>7 form</button></form>',
        f"<button id=l8 onclick=\"var w=window.open('about:blank');w.location='{o}/b8'\">8 popup, then its location</button>",
        '<a id=l9 href="/leave-redirect">9 same-site redirect</a>',
        f"<button id=l10 onclick=\"window.open('{o}/ad1');setTimeout(()=>location.href='{o}/ad2',50)\">10 popup and leave</button>",
    ]
    return ("<!doctype html><meta charset=utf-8><meta name=viewport content='width=device-width'>"
            "<title>Leave</title><style>body{margin:0;font:4vh sans-serif}form{margin:0}"
            "a,button{display:block;width:100%;height:8vh;margin:0;padding:0 8px;box-sizing:border-box;"
            "font:inherit;text-align:left;border:0;border-bottom:1px solid #888;background:#eee;color:#000;"
            "text-decoration:none;line-height:8vh}</style>"
            "<p id=count style='margin:0;height:8vh;line-height:8vh'>clicks 0</p>" + "".join(items) +
            "<script>let n=0;document.addEventListener('click',e=>{if(!e.target.closest('a,button'))return;"
            "n++;count.textContent='clicks '+n;const k=n;setTimeout(()=>fetch('/alive?'+k),1500)},true)</script>")


def popups_word(n):
    """What the address field's popup count says, as shell/src/sdk/sites.js's popupsWord."""
    return "1 popup blocked" if n == 1 else f"{n} popups blocked"
