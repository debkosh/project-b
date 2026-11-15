<script>
  import { shell } from '@/sdk/svelte.svelte.js';
  import { runNotice } from '@/sdk/notice.js';

  // Over a page with nothing to show: Go's sentence, its second line, and its actions as
  // buttons, the primary one first. An alert, flat on the bar's colour.
  let { tab } = $props();
</script>

<div class="notice" use:shell role="alert">
  <p class="title">{tab.notice.title}</p>
  {#if tab.notice.detail}<p class="detail">{tab.notice.detail}</p>{/if}
  {#if tab.notice.actions.length}
    <div class="actions">
      {#each tab.notice.actions as a (a.id)}
        <button class="words" class:primary={a.primary} onclick={() => runNotice(tab, a.id)}>{a.title}</button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .notice {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: 24px;
    box-sizing: border-box;
    background: var(--bar);
    text-align: center;
  }

  p {
    margin: 0;
    line-height: 1.4;
  }

  .title {
    font-size: 15px;
    font-weight: 600;
  }

  .detail {
    max-width: 420px;
    color: var(--muted);
    font: 12px/1.5 var(--mono);
    overflow-wrap: anywhere;
  }

  .actions {
    display: flex;
    gap: 8px;
    margin-top: 8px;
  }
</style>
