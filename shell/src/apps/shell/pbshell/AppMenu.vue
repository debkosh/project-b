<script setup>
import { computed, ref } from 'vue';
import ContextMenu from '@/ui/ContextMenu.vue';
import { keyLabel, requestMenu, runMenuItem, send, state } from '@/sdk/vue.js';

// The ☰ menu: the menu bar's menus for this window and its profile, as Go sends them when
// it opens (docs/menus-shortcuts.md, "The shell's menu"). Nothing in it is the shell's own.
// A submenu's own submenu, such as View's Shell, shows as its items between lines.
const button = ref(null);
const at = ref(null);
const check = '<path d="M3.5 8.5 6.5 11.5 12.5 4.5"/>';

function row(it) {
  return { label: it.title, key: it.action ? keyLabel(it.action) : '', disabled: !it.enabled, icon: it.checked ? check : '', run: () => runMenuItem(it) };
}

function flat(items) {
  const out = [];
  for (const it of items) {
    if (it.separator) out.push({ separator: true });
    else if (it.items) out.push({ separator: true }, ...flat(it.items), { separator: true });
    else out.push(row(it));
  }
  return out.filter((it, i, all) => !(it.separator && (i === 0 || i === all.length - 1 || all[i - 1].separator)));
}

const items = computed(() =>
  state.menu.map((it) => (it.separator ? { separator: true } : it.items ? { label: it.title, items: flat(it.items) } : row(it))),
);

function open() {
  requestMenu();
  const r = button.value.getBoundingClientRect();
  // Its right edge under the button's, as the button sits at the bar's end.
  at.value = { x: r.right, y: r.bottom + 4 };
  send({ t: 'focusShell' });
}
</script>

<template>
  <span class="app-menu">
  <button
    ref="button"
    class="icon-button menu"
    type="button"
    aria-label="Menu"
    title="Menu"
    aria-haspopup="menu"
    :aria-expanded="!!at"
    @click="at ? (at = null) : open()"
  >
    <svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M2.5 4h11M2.5 8h11M2.5 12h11" /></svg>
  </button>
  <ContextMenu v-if="at && items.length" v-shell :items="items" :x="at.x" :y="at.y" label="Menu" @close="at = null" />
  </span>
</template>

<style scoped>
.app-menu {
  display: contents;
}

/* The same 4px apart as the back, forward and reload buttons. */
.menu {
  margin-left: 4px;
}
</style>
