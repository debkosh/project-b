// useTints is tints (tint.js) as a Vue ref, for a computed colour to read so it is made
// again on a new palette: one subscription for every component.

import { shallowRef } from 'vue';
import { tints } from './tint.js';

const version = shallowRef(0);
tints.subscribe((v) => (version.value = v));

export function useTints() {
  return version;
}
