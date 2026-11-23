// Stands in for @duckduckgo/content-scope-scripts' utils, which their autofill-utils.js
// imports for a function the matching never calls.
export function processConfig() {
  throw new Error('processConfig is no part of the field matching');
}
