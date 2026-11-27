<script setup>
import { computed, onMounted, onUnmounted } from 'vue';
import { answerPermission, osLine, osTitle, permissionAsk, permissionGlyph, permissionHost } from '@/sdk/sites.js';
import { pushEscape } from './escape.js';

// A site asking to use its camera, microphone or notifications (docs/daily-driver.md): the
// floating surface of the design system, a dialog named by its question, led by a glyph of
// what is asked. Where Go offers this time, the buttons are Block, Always allow and Allow
// this time, the primary; otherwise Block and Allow. Escape dismisses it. It takes no
// focus, so the page keeps the keyboard; it follows the address field in tab order. The
// root floats over the page: the caller marks it the shell's and places it. With prompt.os,
// the site is allowed but the OS refused its camera or microphone (docs/media-permissions.md):
// the OS's words, then Not now and Open settings, the primary; the page's request waits.
const props = defineProps({ prompt: Object });

const question = computed(() =>
  props.prompt.os ? osTitle(props.prompt) : `${permissionHost(props.prompt)} wants to ${permissionAsk(props.prompt)}`,
);
const answer = (choice) => answerPermission(props.prompt, choice);

// The glyph: a camera, a microphone, a download or a bell.
const glyph = computed(() => permissionGlyph(props.prompt));

let pop = null;
onMounted(() => {
  pop = pushEscape(() => answer('dismiss'));
});
onUnmounted(() => pop());
</script>

<template>
  <div class="ui-permission" role="dialog" :aria-label="question">
    <p class="ui-permission-question">
      <svg class="ui-permission-glyph" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <template v-if="glyph === 'camera'">
          <rect x="1.75" y="4" width="9" height="8" rx="1.5" />
          <path d="M10.75 7 14.25 5v6l-3.5-2" />
        </template>
        <template v-else-if="glyph === 'microphone'">
          <rect x="5.75" y="1.75" width="4.5" height="8" rx="2.25" />
          <path d="M3.5 7.5a4.5 4.5 0 0 0 9 0M8 12v2.25" />
        </template>
        <template v-else-if="glyph === 'download'">
          <path d="M8 2.25v8M4.75 7.25 8 10.5l3.25-3.25M3 13.25h10" />
        </template>
        <template v-else>
          <path d="M4 11.5V7a4 4 0 0 1 8 0v4.5l1 1H3zM6.5 14h3" />
        </template>
      </svg>
      <span v-if="prompt.os"><strong>{{ osTitle(prompt) }}</strong><span class="ui-permission-line">{{ osLine(prompt) }}</span></span>
      <span v-else><strong>{{ permissionHost(prompt) }}</strong> wants to {{ permissionAsk(prompt) }}</span>
    </p>
    <div v-if="prompt.os" class="ui-permission-buttons">
      <button type="button" class="text-button" @click="answer('dismiss')">Not now</button>
      <button type="button" class="text-button primary" @click="answer('settings')">Open settings</button>
    </div>
    <div v-else class="ui-permission-buttons">
      <button type="button" class="text-button" @click="answer('block')">Block</button>
      <template v-if="prompt.once">
        <button type="button" class="text-button" @click="answer('allow')">Always allow</button>
        <button type="button" class="text-button primary" @click="answer('once')">Allow this time</button>
      </template>
      <button v-else type="button" class="text-button primary" @click="answer('allow')">Allow</button>
    </div>
  </div>
</template>

<style>
.ui-permission {
  width: 360px;
  max-width: calc(100vw - 16px);
  padding: 16px;
  box-sizing: border-box;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  box-shadow: var(--shadow-float);
  color: var(--text);
}

.ui-permission-line {
  display: block;
  margin-top: 4px;
  color: var(--muted);
}

.ui-permission-question {
  display: flex;
  gap: 10px;
  margin: 0 0 16px;
  font-size: calc(13rem / 16);
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.ui-permission-glyph {
  flex: none;
  margin-top: 1px;
  color: var(--icon);
}

.ui-permission-question strong {
  font-weight: 600;
}

.ui-permission-buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
