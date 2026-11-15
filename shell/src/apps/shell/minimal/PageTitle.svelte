<script>
  import { activeTab, send, state as go, tabLabel } from '@/sdk/svelte.svelte.js';

  // One of our tabs (docs/shells.md, rule 19): its icon and title where the address goes,
  // and a way to close it, with no field, as an address can't take it anywhere.
  const tab = $derived(activeTab(go));
  const title = $derived(tabLabel(tab));
</script>

<div class="page-title">
  {#if tab.favicon}<img src={tab.favicon} alt="" width="16" height="16" />{/if}
  <span class="title">{title}</span>
  <button class="btn" aria-label="Close {title}" title="Close {title}" onclick={() => send({ t: 'closeTab', id: tab.id })}>
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="m4 4 8 8M12 4l-8 8" /></svg>
  </button>
</div>

<style>
  /* The address field's box, with the title in it and the close at its end. */
  .page-title {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
    min-width: 0;
    height: 28px;
    margin: 0 6px;
    padding-left: 10px;
    box-sizing: border-box;
    border: 1px solid var(--line);
    background: var(--surface);
  }

  img {
    flex: none;
  }

  .title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* Inside the hairline. */
  .btn {
    width: 26px;
    height: 26px;
  }
</style>
