<script>
  import { send, setPref, shell, state } from '@/sdk/svelte.svelte.js';

  // A release ready to install says so once (docs/updates.md): a line over the toolbar, as
  // the crash bar is, with Later and Install. Either one records the version as seen in the
  // profile's prefs (updateNoticed), so it shows once a release, not at every start; the
  // menu's Install update and Settings › About still offer it.
  const ready = $derived(state.update?.phase === 'ready');
  const version = $derived(ready ? state.update.version || '' : '');
  const shown = $derived(ready && state.prefs?.updateNoticed !== version);

  function later() {
    setPref('updateNoticed', version);
  }

  function install() {
    setPref('updateNoticed', version);
    send({ t: 'relaunch' });
  }
</script>

{#if shown}
  <div class="update" use:shell role="dialog" aria-label="Update ready">
    <span class="words" role="status">{version ? `Project B ${version} is ready` : 'An update is ready'}</span>
    <button class="text-button quiet" onclick={later}>Later</button>
    <button class="text-button primary" onclick={install}>Install</button>
  </div>
{/if}

<style>
  .update {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    height: 64px;
    padding: 0 12px 0 16px;
    background: var(--bar);
    border-top: 1px solid var(--line);
  }

  .words {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-weight: 500;
  }

  button {
    height: 48px;
    padding: 0 16px;
  }
</style>
