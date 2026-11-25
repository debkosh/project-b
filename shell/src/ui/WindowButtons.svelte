<script>
  import { send, state as go } from '@/sdk/svelte.svelte.js';

  // The window's own buttons where the frame is ours, Linux and Windows, for the Svelte
  // shells: WindowButtons.vue's (docs/design-system.md, "The window's own buttons and
  // menus"). Minimise, maximise and close at the right end of the top row; the Mac keeps its
  // own, and this draws nothing there. Each runs Go's action, as a menu would.
  const ours = $derived(go.platform !== 'mac' && go.platform !== 'android' && !(go.controls && go.controls.w > 0));
  const run = (action) => send({ t: 'run', text: action });
</script>

{#if ours}
  <div class="window-buttons">
    <button class="icon-button" type="button" aria-label="Minimise" title="Minimise" onclick={() => run('window.minimize')}>
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3.5 8h9" /></svg>
    </button>
    <button class="icon-button" type="button" aria-label="Maximise" title="Maximise" onclick={() => run('window.zoom')}>
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3.75" y="3.75" width="8.5" height="8.5" rx="1" /></svg>
    </button>
    <button class="icon-button close" type="button" aria-label="Close" title="Close" onclick={() => run('window.close')}>
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M4 4l8 8M12 4l-8 8" /></svg>
    </button>
  </div>
{/if}

<style>
  .window-buttons {
    display: flex;
    flex: none;
    align-items: center;
    margin-left: 8px;
  }

  .window-buttons .icon-button {
    width: 32px;
    height: 28px;
    color: var(--icon);
  }

  .window-buttons .close:hover {
    background: var(--danger);
    color: #fff;
  }
</style>
