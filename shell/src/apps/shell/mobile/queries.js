// One count for every suggest this shell sends, so each answer reaches the part that asked:
// the address field, or the new tab page.
let last = 0;

export function nextQuery() {
  return ++last;
}
