<script setup>
import { computed, onMounted, reactive, ref, watch, watchEffect } from 'vue';
import { paintTheme } from '@/theme.js';
import BrowserIcon from '@/ui/BrowserIcon.vue';
import ImportChoices from '@/ui/ImportChoices.vue';
import { broughtLines, choicesOf, holds } from '@/ui/imports.js';
import Segmented from '@/ui/Segmented.vue';
import Swatches from '@/ui/Swatches.vue';
import { framePresets } from '@/ui/colours.js';
import Preview from './Preview.vue';
import { send, setPref, state } from './state.js';

// The welcome (docs/onboarding.md): a few steps, each skippable, whose every choice is a
// pref set at once, so the page, its window's frame and the picture of the browser take it
// together. Over its whole window, as a first start opens it, it starts with a splash.

watchEffect(() => paintTheme(state.tones, state.theme));
document.title = 'Welcome';

// Bring your things over is a step while Go looks for other browsers, and stays one when
// it finds a profile; a phone keeps each app's data from the others, so it has none.
const steps = computed(() => {
  const bring = !state.phone && (state.browsers === null || state.browsers.length > 0);
  return ['hello', ...(bring ? ['bring'] : []), 'style', 'protection', 'done'];
});
const step = ref('hello');
const at = computed(() => steps.value.indexOf(step.value));
const go = (by) => (step.value = steps.value[Math.min(steps.value.length - 1, Math.max(0, at.value + by))]);
const finish = () => send({ t: 'done' });
// The list came back empty while the step showed: the next one takes its place.
watch(steps, (s) => {
  if (!s.includes(step.value)) step.value = 'style';
});

// Bring your things over: the rows turned on, by key, each with what is ticked in it, all
// it holds at first. Bring over sends each, and the step then shows what each brought.
const chosen = reactive({});
function choose(p, on) {
  if (on) chosen[p.key] = choicesOf(p).map((c) => c.id);
  else delete chosen[p.key];
}
const picked = computed(() => Object.keys(chosen).filter((k) => chosen[k].length));
const brought = ref([]);
function bringOver() {
  for (const key of picked.value) send({ t: 'import', key, what: chosen[key] });
  brought.value = (state.browsers || []).filter((p) => picked.value.includes(p.key));
}

// The splash plays once, when the welcome has its window to itself; less motion skips it.
const splash = ref(false);
const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
watch(
  () => state.ready,
  (ready) => {
    if (!ready || !state.whole || reduced) return;
    splash.value = true;
    setTimeout(() => (splash.value = false), 1800);
  },
  { once: true },
);

// The way to work: a shell, and for Standard where its tabs go.
const layout = computed(() => {
  const p = state.prefs;
  return state.layouts.find((l) => l.shell === p.shell && (!l.tabLayout || l.tabLayout === p.tabLayout))?.id || 'side';
});
function chooseLayout(id) {
  const l = state.layouts.find((l) => l.id === id);
  if (!l) return;
  if (l.tabLayout) setPref('tabLayout', l.tabLayout);
  setPref('shell', l.shell);
}

const themes = [
  { value: 'system', label: 'System' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
];
const positions = [
  { value: 'top', label: 'Top' },
  { value: 'bottom', label: 'Bottom' },
];

// The colours the hello's window turns through while it waits: the presets with a colour.
const cycle = framePresets.filter((p) => p.color).map((p) => p.color);
const turn = ref(0);
if (!reduced) setInterval(() => (turn.value = (turn.value + 1) % cycle.length), 2400);

// Focus goes to each step's heading, so a screen reader reads where it is.
const heading = ref(null);
watch([step, brought], () => setTimeout(() => heading.value?.focus(), 0));
onMounted(() => heading.value?.focus());
</script>

<template>
  <div class="welcome" :class="{ whole: state.whole, phone: state.phone }">
    <div v-if="splash" class="splash" aria-hidden="true">
      <span class="mark"><svg viewBox="0 0 32 32"><rect x="7" y="9" width="18" height="14" rx="3" /><path d="M16 9v14" /></svg></span>
      <span class="name">Project B</span>
    </div>

    <main v-if="state.ready" class="content">
      <!-- Hello: the mark, the name, one line, and the way in or past. -->
      <section v-if="step === 'hello'" class="hello" aria-labelledby="hello">
        <span class="mark" aria-hidden="true"><svg viewBox="0 0 32 32"><rect x="7" y="9" width="18" height="14" rx="3" /><path d="M16 9v14" /></svg></span>
        <h1 id="hello" ref="heading" tabindex="-1">Welcome to Project B</h1>
        <p class="line">A browser you shape to fit.</p>
        <div class="actions">
          <button type="button" class="text-button primary" @click="go(1)">Get started</button>
          <button type="button" class="text-button quiet" @click="finish">Skip setup</button>
        </div>
        <div class="waiting" aria-hidden="true" :style="{ '--frame': cycle[turn], '--accent-soft': 'var(--line)' }">
          <Preview :layout="state.phone ? 'phone' : 'side'" />
        </div>
      </section>

      <!-- Bring your things over: each other browser's profile, turned on to choose what
           comes, then what each brought. -->
      <section v-else-if="step === 'bring'" class="bring" aria-labelledby="bring">
        <template v-if="!brought.length">
          <h1 id="bring" ref="heading" tabindex="-1">Bring your things over</h1>
          <p class="sub">Found on this Mac. Nothing leaves it.</p>
          <div class="card">
            <div v-if="state.browsers === null" class="row"><span class="state">Looking for other browsers…</span></div>
            <div v-for="p in state.browsers || []" :key="p.key" class="browser">
              <label class="row">
                <BrowserIcon :icon="p.icon" :title="p.title" />
                <span class="what">
                  <span class="label">{{ p.title }}</span>
                  <span class="state">{{ holds(p) }}</span>
                </span>
                <input
                  type="checkbox"
                  role="switch"
                  class="switch"
                  :aria-label="`Bring over from ${p.title}`"
                  :checked="!!chosen[p.key]"
                  :aria-checked="String(!!chosen[p.key])"
                  :disabled="!choicesOf(p).length"
                  @change="choose(p, $event.target.checked)"
                />
              </label>
              <div v-if="chosen[p.key]" class="open">
                <ImportChoices v-model="chosen[p.key]" :profile="p" />
              </div>
            </div>
          </div>
        </template>
        <template v-else>
          <h1 id="bring" ref="heading" tabindex="-1">Brought over</h1>
          <p class="sub">Bookmarks come in a folder of their own, on the Bookmarks page.</p>
          <div class="card">
            <div v-for="p in brought" :key="p.key" class="browser">
              <div class="row">
                <BrowserIcon :icon="p.icon" :title="p.title" />
                <span class="what">
                  <span class="label">{{ p.title }}</span>
                  <span v-if="!state.imported[p.key]" class="state">Bringing over…</span>
                </span>
              </div>
              <ul v-if="state.imported[p.key]" class="lines" :aria-label="`Brought over from ${p.title}`">
                <li v-for="(l, i) in broughtLines(state.imported[p.key], state.searchEngines)" :key="i" :class="{ problem: l.problem }">
                  <svg v-if="!l.problem" viewBox="0 0 16 16" aria-hidden="true"><path d="M4 8.5l2.5 2.5L12 5.5" /></svg>
                  <span>{{ l.text }}</span>
                  <span v-if="l.note" class="state">{{ l.note }}</span>
                </li>
              </ul>
            </div>
          </div>
        </template>
      </section>

      <!-- Style: the picture beside the choices, each taken at once. -->
      <section v-else-if="step === 'style'" class="style" aria-labelledby="style">
        <h1 id="style" ref="heading" tabindex="-1">Pick your style</h1>
        <div class="split">
          <div class="picture">
            <Preview :layout="state.phone ? 'phone' : layout" :top="state.prefs.addressBarPosition === 'top'" />
          </div>
          <div class="choices">
            <div v-if="!state.phone" class="choice">
              <span class="label" id="layout-label">Tabs</span>
              <div class="layouts" role="radiogroup" aria-labelledby="layout-label">
                <button
                  v-for="l in state.layouts"
                  :key="l.id"
                  type="button"
                  role="radio"
                  class="layout"
                  :aria-checked="layout === l.id"
                  @click="chooseLayout(l.id)"
                >
                  <Preview :layout="l.id" />
                  <span>{{ l.name }}</span>
                </button>
              </div>
            </div>
            <div v-else class="choice">
              <span class="label">Address bar</span>
              <Segmented label="Address bar" :options="positions" :model-value="state.prefs.addressBarPosition" @update:model-value="setPref('addressBarPosition', $event)" />
            </div>
            <div class="choice">
              <span class="label">Colour</span>
              <Swatches label="Colour" custom :colors="framePresets" :model-value="state.prefs.frameColor" @update:model-value="setPref('frameColor', $event)" />
            </div>
            <div class="choice">
              <span class="label">Theme</span>
              <Segmented label="Theme" :options="themes" :model-value="state.prefs.theme" @update:model-value="setPref('theme', $event)" />
            </div>
          </div>
        </div>
      </section>

      <!-- Protection: what keeps the web in check, and the search engine. -->
      <section v-else-if="step === 'protection'" class="protection" aria-labelledby="protection">
        <h1 id="protection" ref="heading" tabindex="-1">Protection</h1>
        <div class="card">
          <div class="row">
            <span class="label">Ad blocker</span>
            <span class="state">On</span>
          </div>
          <label class="row">
            <span class="label">Search engine</span>
            <select :value="state.prefs.searchEngine" @change="send({ t: 'setSearchEngine', value: $event.target.value })">
              <option v-for="e in state.searchEngines" :key="e.id" :value="e.id">{{ e.name }}</option>
            </select>
          </label>
          <label class="row">
            <span class="label">Automatic updates</span>
            <input
              type="checkbox"
              role="switch"
              class="switch"
              :checked="state.autoUpdate"
              :aria-checked="String(state.autoUpdate)"
              @change="send({ t: 'setAutoUpdate', on: $event.target.checked })"
            />
          </label>
        </div>
      </section>

      <!-- Done: three things worth knowing, and the way in. -->
      <section v-else class="done" aria-labelledby="done">
        <h1 id="done" ref="heading" tabindex="-1">You're set</h1>
        <ul class="tips">
          <li><b>Type anywhere</b><span>An address or a search, in the box a new tab opens.</span></li>
          <li><b>Each site's own settings</b><span>From the address: what the site may use, and its ads.</span></li>
          <li><b>Workspaces</b><span>Sets of tabs, each with its own sign-ins.</span></li>
        </ul>
        <div class="card default">
          <div class="row">
            <span class="label">Default browser</span>
            <span v-if="state.isDefault" class="state">This is your default browser</span>
            <button v-else type="button" class="text-button" @click="send({ t: 'makeDefault' })">Make default browser</button>
          </div>
          <div v-if="state.phone" class="row">
            <span class="label">Download notifications</span>
            <span v-if="state.notifications" class="state">On</span>
            <button v-else type="button" class="text-button" @click="send({ t: 'askNotifications' })">Allow</button>
          </div>
        </div>
        <div class="actions">
          <button type="button" class="text-button primary" @click="finish">Start browsing</button>
        </div>
      </section>
    </main>

    <!-- The way through: where it is, back, and on. -->
    <footer v-if="state.ready && step !== 'hello' && step !== 'done'" class="nav">
      <button type="button" class="text-button quiet" @click="go(-1)">Back</button>
      <ol class="dots" aria-label="Steps">
        <li v-for="(s, i) in steps" :key="s" :class="{ on: i === at }" :aria-current="i === at ? 'step' : undefined"></li>
      </ol>
      <div class="ahead">
        <button type="button" class="text-button quiet" @click="finish">Skip setup</button>
        <button v-if="step === 'bring' && !brought.length && picked.length" type="button" class="text-button primary" @click="bringOver">Bring over</button>
        <button v-else type="button" class="text-button primary" @click="go(1)">Continue</button>
      </div>
    </footer>
  </div>
</template>

<style>
body {
  background: var(--bar);
  transition: background var(--motion) var(--ease);
}
</style>

<style scoped>
.welcome {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 100%;
  box-sizing: border-box;
  padding: 24px;
}

/* Over the whole window the top is the window's: its buttons, and the strip it moves by. */
.welcome.whole {
  padding-top: 48px;
}

.content {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  width: 100%;
  max-width: 880px;
  margin: 0 auto;
}

h1 {
  margin: 0 0 24px;
  font-size: 24px;
  font-weight: 600;
  outline: none;
}

.mark {
  display: grid;
  place-items: center;
  width: 64px;
  height: 64px;
  border-radius: 16px;
  background: var(--accent);
  transition: background var(--motion) var(--ease);
}

.mark svg {
  width: 32px;
  height: 32px;
  fill: none;
  stroke: var(--on-accent);
  stroke-width: 1.5;
  stroke-linejoin: round;
}

/* The splash: the mark draws itself and the name fades in, then the welcome comes. */
.splash {
  position: fixed;
  inset: 0;
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  background: var(--bar);
  animation: splash-out 300ms var(--ease) 1500ms forwards;
}

.splash .mark {
  width: 88px;
  height: 88px;
  border-radius: 22px;
  animation: grow 500ms var(--ease) both;
}

.splash .mark svg {
  width: 44px;
  height: 44px;
}

.splash .mark rect,
.splash .mark path {
  stroke-dasharray: 64;
  stroke-dashoffset: 64;
  animation: draw 700ms var(--ease) 300ms forwards;
}

.splash .mark path {
  animation-delay: 700ms;
}

.splash .name {
  font-size: 20px;
  font-weight: 600;
  opacity: 0;
  animation: fade-in 400ms var(--ease) 800ms forwards;
}

@keyframes grow {
  from {
    transform: scale(0.6);
    opacity: 0;
  }
}

@keyframes draw {
  to {
    stroke-dashoffset: 0;
  }
}

@keyframes fade-in {
  to {
    opacity: 1;
  }
}

@keyframes splash-out {
  to {
    opacity: 0;
    visibility: hidden;
  }
}

/* Hello: the words high up, and a window rising from the foot of the page, its frame
   turning through the colours as it waits. */
.content:has(> .hello) {
  justify-content: flex-start;
}

.hello {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding-top: 72px;
}

.hello .mark {
  margin-bottom: 24px;
}

.hello h1 {
  margin-bottom: 8px;
  font-size: 32px;
}

.line {
  margin: 0 0 24px;
  color: var(--muted);
  font-size: 15px;
}

.welcome:has(.hello) {
  height: 100%;
  overflow: hidden;
}

.waiting {
  width: min(900px, 100%);
  margin-top: 56px;
}

.waiting :deep(.window) {
  box-shadow: 0 16px 48px rgba(16, 24, 40, 0.12);
}

.waiting :deep(.window),
.waiting :deep(.phone) {
  transition: border-color 600ms var(--ease), background-color 600ms var(--ease);
}

.actions {
  display: flex;
  gap: 8px;
  justify-content: center;
}

.split {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr);
  gap: 32px;
  align-items: start;
}

.phone .split {
  grid-template-columns: 1fr;
}

.choices {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.choice {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.label {
  font-weight: 500;
}

.layouts {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.layout {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 8px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  color: inherit;
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  transition: border-color var(--motion) var(--ease);
}

.layout :deep(.window) {
  border-width: 2px;
  border-radius: 6px;
}

.layout[aria-checked='true'] {
  border-color: var(--accent);
  box-shadow: inset 0 0 0 1px var(--accent);
}

.layout:focus-visible {
  outline: none;
  box-shadow: var(--focus-ring);
}

.card {
  display: flex;
  flex-direction: column;
  max-width: 560px;
  border-radius: 8px;
  background: var(--surface);
  box-shadow: inset 0 0 0 1px var(--line);
}

.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 48px;
  padding: 0 16px;
}

.row + .row {
  border-top: 1px solid var(--line);
}

.state {
  color: var(--muted);
}

.tips {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin: 0 0 32px;
  padding: 0;
  list-style: none;
}

.tips li {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.tips span {
  color: var(--muted);
}

.done .actions {
  justify-content: flex-start;
}

.card.default {
  margin-bottom: 32px;
}

.nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  max-width: 880px;
  margin: 24px auto 0;
}

.dots {
  display: flex;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.dots li {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--line);
  transition: background var(--motion) var(--ease);
}

.dots li.on {
  background: var(--accent);
}

.ahead {
  display: flex;
  gap: 8px;
}

/* Bring your things over: a row for each profile, turned on to open what it brings. */
.bring .card {
  max-width: 640px;
  overflow: hidden;
  border: 1px solid var(--line);
  box-shadow: none;
}

.sub {
  margin: -16px 0 24px;
  color: var(--muted);
}

.browser + .browser {
  border-top: 1px solid var(--line);
}

.browser .row {
  justify-content: flex-start;
  gap: 12px;
  min-height: 56px;
}

.what {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.what .state {
  font-size: 12px;
}

.open {
  padding: 12px 16px 12px 60px;
  border-top: 1px solid var(--line);
  background: var(--bar);
}

.lines {
  margin: 0;
  padding: 0 16px 12px 60px;
  list-style: none;
}

.lines li {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 28px;
}

.lines svg {
  flex: none;
  width: 16px;
  height: 16px;
  fill: none;
  stroke: var(--accent);
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.lines .state {
  margin-left: auto;
  font-size: 12px;
}

.lines .problem {
  color: var(--danger);
}

/* A phone: one column, the way through at the thumb. */
.phone .content {
  justify-content: flex-start;
}
</style>
