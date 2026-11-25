<script setup>
import { runNotice } from '@/sdk/notice.js';

// Over a page that failed, crashed, was blocked or has a bad certificate, which otherwise
// shows blank (docs/daily-driver.md): Go's sentence, its second line, and its actions as
// buttons, the primary one first. An alert, on the bar's colour, filling its place: the
// caller puts it over the tab's page and marks it the shell's.
defineProps({ tab: Object });
</script>

<template>
  <div class="ui-notice" role="alert" :data-kind="tab.notice.kind">
    <svg class="ui-notice-mark" width="32" height="32" viewBox="0 0 32 32" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <path d="M16 5 29 27H3z" />
      <path d="M16 13v7M16 23.5v.01" />
    </svg>
    <p class="ui-notice-title">{{ tab.notice.title }}</p>
    <p v-if="tab.notice.detail" class="ui-notice-detail">{{ tab.notice.detail }}</p>
    <div v-if="tab.notice.actions.length" class="ui-notice-actions">
      <button
        v-for="a in tab.notice.actions"
        :key="a.id"
        type="button"
        class="text-button"
        :class="{ primary: a.primary }"
        @click="runNotice(tab, a.id)"
      >{{ a.title }}</button>
    </div>
  </div>
</template>

<style>
.ui-notice {
  position: absolute;
  inset: 0;
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 24px;
  box-sizing: border-box;
  background: var(--bar);
  color: var(--text);
  text-align: center;
}

.ui-notice-mark {
  color: var(--muted);
}

.ui-notice[data-kind='certificate'] .ui-notice-mark {
  color: var(--danger);
}

.ui-notice-title {
  margin: 8px 0 0;
  font-size: calc(15rem / 16);
  font-weight: 600;
}

.ui-notice-detail {
  max-width: 420px;
  margin: 0;
  color: var(--muted);
  font-size: calc(13rem / 16);
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.ui-notice-actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}
</style>
