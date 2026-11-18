// Search in settings (docs/settings.md): what was typed, and the rows it finds. Each row
// says whether it matches, by its label, its second line or its section's name, and every
// row on the page is counted here, so a layout can count the hits in each section.

import { computed, reactive, ref } from 'vue';

export const query = ref('');
export const words = computed(() => query.value.trim().toLowerCase());
export const searching = computed(() => words.value !== '');

// Every row drawn now, each {section, hit}, hit a function saying whether it matches.
const rows = reactive(new Set());

export function addRow(row) {
  rows.add(row);
  return () => rows.delete(row);
}

// The hits in each section, by its id, and in all.
export const hits = computed(() => {
  const by = {};
  let all = 0;
  for (const r of rows) {
    if (!r.hit()) continue;
    by[r.section] = (by[r.section] || 0) + 1;
    all++;
  }
  return { by, all };
});

// matches says whether any of the texts has what was typed, in any case.
export const matches = (...texts) => !searching.value || texts.some((t) => t && t.toLowerCase().includes(words.value));

// marked splits text at what was typed, for the page to mark each match.
export function marked(text) {
  if (!searching.value || !text) return [{ text: text || '', hit: false }];
  const out = [];
  const lower = text.toLowerCase();
  let at = 0;
  for (let i = lower.indexOf(words.value); i >= 0; i = lower.indexOf(words.value, at)) {
    if (i > at) out.push({ text: text.slice(at, i), hit: false });
    out.push({ text: text.slice(i, i + words.value.length), hit: true });
    at = i + words.value.length;
  }
  if (at < text.length) out.push({ text: text.slice(at), hit: false });
  return out;
}
