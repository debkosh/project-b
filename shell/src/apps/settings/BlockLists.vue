<script setup>
import { computed, ref } from 'vue';
import { phone } from '@/phone.js';
import { clause, groupName, homeHost, listMeta } from './blocking.js';
import { send, state } from './state.js';

// The Block lists page (docs/blocking.md, ux's review): the groups, each a row with its
// switch and a line of what it blocks and how many of its lists are on; opened, its lists,
// each with its own switch. The Safe Browsing groups are under Dangerous sites, greyed
// while Warn before dangerous sites is off. Go keeps the test group out, and owns what a
// group's switch does (setBlockGroup): this page sends it and shows what comes back.
// On a phone a tap on a group's words opens it, and a hairline before its switch keeps
// the two apart.

const network = computed(() => state.blocking.groups.filter((g) => g.blocker !== 'safe'));
const safe = computed(() => state.blocking.groups.filter((g) => g.blocker === 'safe'));
const warn = computed(() => !!state.prefs.warnDangerous);

// The groups opened, by id.
const opened = ref({});
const toggle = (id) => (opened.value = { ...opened.value, [id]: !opened.value[id] });

const setGroup = (g, on) => send({ t: 'setBlockGroup', id: g.id, on });
const setList = (l, on) => send({ t: 'setBlockList', id: l.id, on });

const intro = phone
  ? 'Public lists of what to block, updated every hour. Tap a group to choose its lists.'
  : 'Public lists of what to block, updated every hour. Turn a group on or off, or open it to choose its lists.';
</script>

<template>
  <div class="block-lists" :class="{ phone }">
    <p class="intro">{{ intro }}</p>
    <template v-for="part in [{ id: 'network', groups: network }, { id: 'safe', groups: safe }]" :key="part.id">
      <section v-if="part.groups.length" :class="part.id" :aria-labelledby="part.id === 'safe' ? 'dangerous-sites' : undefined" :aria-label="part.id === 'safe' ? undefined : 'Block lists'">
        <template v-if="part.id === 'safe'">
          <h2 id="dangerous-sites">Dangerous sites</h2>
          <p class="intro under" :class="{ off: !warn }">{{ warn ? 'Used while Warn before dangerous sites is on.' : 'Turn on Warn before dangerous sites to use these' }}</p>
        </template>
        <div class="card" :class="{ greyed: part.id === 'safe' && !warn }">
          <template v-for="g in part.groups" :key="g.id">
            <div class="row group" :data-group="g.id">
              <button
                type="button"
                class="opens"
                :aria-label="part.id === 'safe' ? groupName(g) : g.title"
                :aria-describedby="`group-${g.id}-line`"
                :aria-expanded="String(!!opened[g.id])"
                @click="toggle(g.id)"
              >
                <svg v-if="!phone" class="chevron" :class="{ open: opened[g.id] }" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6 3.5 10.5 8 6 12.5" /></svg>
                <span class="label">
                  <span class="name">{{ part.id === 'safe' ? groupName(g) : g.title }}</span>
                  <!-- The count never breaks: a long line wraps before it. -->
                  <span :id="`group-${g.id}-line`" class="secondary">{{ clause(g.about) }} <span class="count">· {{ g.onCount }} of {{ g.total }} on</span></span>
                </span>
              </button>
              <span v-if="phone" class="divider" aria-hidden="true"></span>
              <input
                type="checkbox"
                role="switch"
                class="switch"
                :aria-label="part.id === 'safe' ? groupName(g) : g.title"
                :aria-describedby="`group-${g.id}-line`"
                :checked="g.on"
                :aria-checked="String(g.on)"
                @change="setGroup(g, $event.target.checked)"
              />
            </div>
            <template v-if="opened[g.id]">
              <div v-for="l in g.lists" :key="l.id" class="row list" :data-list="l.id">
                <span class="label">
                  <span class="name">{{ l.title }}</span>
                  <span :id="`list-${l.id}-line`" class="secondary">{{ clause(l.about) }} · {{ homeHost(l.home) }}</span>
                </span>
                <span class="meta">{{ listMeta(l) }}</span>
                <input
                  type="checkbox"
                  role="switch"
                  class="switch"
                  :aria-label="l.title"
                  :aria-describedby="`list-${l.id}-line`"
                  :checked="l.on"
                  :aria-checked="String(l.on)"
                  @change="setList(l, $event.target.checked)"
                />
              </div>
            </template>
          </template>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.intro {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: calc(13rem / 16);
}

.intro.under {
  margin: 0;
  font-size: calc(12rem / 16);
}

.block-lists > section {
  margin-top: 16px;
}

/* A group's row: its words are the button that opens it, the switch only switches. */
.row.group,
.row.list {
  flex-wrap: nowrap;
  gap: 12px;
}

.opens {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 12px;
  align-self: stretch;
  padding: 0;
  border: 0;
  background: none;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.opens:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
  border-radius: 4px;
}

.chevron {
  flex: none;
  color: var(--icon);
  transition: transform var(--motion) var(--ease);
}

.chevron.open {
  transform: rotate(90deg);
}

.count {
  white-space: nowrap;
}

.divider {
  flex: none;
  align-self: stretch;
  width: 1px;
  margin: 4px 0;
  background: var(--line);
}

/* A group's lists sit under it, indented past the chevron. */
.row.list {
  padding-left: 44px;
}

.meta {
  flex: none;
  color: var(--muted);
  font-size: calc(12rem / 16);
  white-space: nowrap;
}

/* Dangerous sites while Warn before dangerous sites is off: greyed, still to be chosen. */
.greyed {
  opacity: 0.5;
}

/* With no chevron on a phone, a list sits a little in from its group. */
.phone .row.list {
  padding-left: 28px;
}
</style>
