<script setup>
// Full Disk Access, which macOS keeps Safari's files behind (docs/onboarding.md): a picture
// of the switch to turn on in System Settings, the steps, and a line that waits while Go
// tries the files each second. macOS lists the app there, off, once it has tried them, and
// once it is turned on offers to quit it; the access works without, so the steps say Later. The welcome's Safari row and Settings' Import show it;
// whoever shows it has already opened System Settings, and Open System Settings opens it
// again.
defineEmits(['open']);
</script>

<template>
  <div class="ui-allow-access">
    <div class="ui-allow-card how">
      <div class="picture" aria-hidden="true">
        <span class="path">Privacy &amp; Security › Full Disk Access</span>
        <span class="app">
          <span class="mark"><svg viewBox="0 0 32 32"><rect x="7" y="9" width="18" height="14" rx="3" /><path d="M16 9v14" /></svg></span>
          <span class="name">Project B</span>
          <span class="toggle"></span>
        </span>
      </div>
      <ol class="steps">
        <li>Turn on <strong>Project B</strong></li>
        <li>If macOS offers to quit it, choose <strong>Later</strong></li>
        <li>Come back here</li>
      </ol>
    </div>
    <div class="ui-allow-card waiting" role="status">
      <span class="spinner" aria-hidden="true"></span>
      <span class="label">Waiting for access…</span>
      <button type="button" class="text-button" @click="$emit('open')">Open System Settings</button>
    </div>
  </div>
</template>

<style>
.ui-allow-access {
  container-type: inline-size;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ui-allow-card {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 20px 24px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--surface);
}

.ui-allow-access .picture {
  flex: none;
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 280px;
  max-width: 100%;
  padding: 12px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--bar);
  font-size: calc(12rem / 16);
  color: var(--muted);
}

.ui-allow-access .app {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--surface);
  color: var(--text);
}

.ui-allow-access .mark {
  display: grid;
  place-items: center;
  width: 20px;
  height: 20px;
  border-radius: 5px;
  background: var(--accent);
}

.ui-allow-access .mark svg {
  width: 16px;
  height: 16px;
  fill: none;
  stroke: var(--on-accent);
  stroke-width: 2;
  stroke-linejoin: round;
}

.ui-allow-access .name {
  flex: 1;
}

/* A switch, on, as System Settings draws it. */
.ui-allow-access .toggle {
  position: relative;
  width: 32px;
  height: 18px;
  border-radius: 9px;
  background: #0a84ff;
}

.ui-allow-access .toggle::after {
  content: '';
  position: absolute;
  top: 2px;
  right: 2px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #fff;
}

.ui-allow-access .steps {
  margin: 0;
  padding-left: 20px;
  line-height: 1.9;
  color: var(--text);
}

.ui-allow-access .waiting {
  gap: 12px;
  padding: 12px 16px;
}

.ui-allow-access .waiting {
  flex-wrap: wrap;
}

.ui-allow-access .waiting .label {
  flex: 1 0 auto;
  white-space: nowrap;
  color: var(--text);
}

.ui-allow-access .spinner {
  flex: none;
  width: 18px;
  height: 18px;
  border: 2px solid var(--line);
  border-top-color: var(--muted);
  border-radius: 50%;
  animation: ui-allow-spin 0.9s linear infinite;
}

@media (prefers-reduced-motion: reduce) {
  .ui-allow-access .spinner {
    animation-duration: 3s;
  }
}

@keyframes ui-allow-spin {
  to {
    transform: rotate(360deg);
  }
}

/* Narrow, as in a dialog or on a small window: the steps under the picture. */
@container (max-width: 520px) {
  .ui-allow-access .how {
    flex-direction: column;
    align-items: stretch;
    gap: 16px;
    padding: 16px;
  }

  .ui-allow-access .picture {
    width: auto;
  }
}
</style>
