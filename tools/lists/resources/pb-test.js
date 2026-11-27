// The test list's scriptlet and stand-in script (core/blocking/testlist.txt): marks the page's
// root element, so a check can see that it ran before the page's own scripts.
(function () {
  document.documentElement.setAttribute('data-pb-test-scriptlet', 'ran');
})();
