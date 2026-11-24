<script>
  import { crashLine, crashPaths, send, shell, state } from '@/sdk/svelte.svelte.js';

  // The last run's crash, in one line under the toolbar until it is dismissed: where its
  // log is, and macOS's report on pointing at it.
</script>

{#if state.crashLog}
  <div class="crash" use:shell title={crashPaths(state)}>
    <span role="status">Project B crashed last time. {crashLine(state)}</span>
    <button class="words" onclick={() => send({ t: 'dismissCrash' })}>Dismiss</button>
  </div>
{/if}

<style>
  .crash {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: none;
    height: 36px;
    padding: 0 4px 0 12px;
    background: var(--bar);
    border-bottom: 1px solid var(--line);
    font-size: 12px;
  }

  span {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
</style>
