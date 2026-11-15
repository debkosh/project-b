<script>
  import { controlSpace, drag, send, shell, state } from '@/sdk/svelte.svelte.js';
  import { hint } from './hint.js';
  import { TAB, topRow } from './rows.js';
  import TabList from './TabList.svelte';

  // The tabs on top: the native window buttons' space, the tabs, New Tab, then empty space
  // that moves the window, never less than 48px. The row is as tall as centring the window
  // buttons takes, and the tabs sit at its foot, joining the toolbar; the space above them
  // moves the window too. The tabs scroll sideways, and the SDK doesn't measure on a scroll,
  // so the bar is the shell's and only its empty spaces drag.
  const row = $derived(topRow(state));
</script>

<div class="tabbar" use:shell style:height="{row + 1}px">
  {#if row > TAB}
    <div class="lift" use:drag style:left={controlSpace(state).left} style:height="{row - TAB}px"></div>
  {/if}
  <div class="space" use:drag style:width={controlSpace(state).left}></div>
  <TabList tabs={state.tabs} active={state.active} />
  <button class="new" aria-label="New Tab" title={hint('New Tab', 'tab.new')} onclick={() => send({ t: 'newTab' })}>
    <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 2v12M2 8h12" /></svg>
  </button>
  <div class="drag" use:drag></div>
</div>

<style>
  /* 30px of tabs at the row's foot and a hairline under them, which the active tab covers
     to join the toolbar. */
  .tabbar {
    position: relative;
    display: flex;
    align-items: flex-end;
    flex: none;
    background: var(--tabbar);
    box-shadow: inset 0 -1px var(--line);
  }

  .lift {
    position: absolute;
    top: 0;
    right: 0;
  }

  .space {
    flex: none;
    align-self: stretch;
  }

  .new {
    flex: none;
    display: grid;
    place-items: center;
    width: 34px;
    height: 30px;
    margin-bottom: 1px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--icon);
  }

  .new:hover {
    background: var(--hover);
  }

  .new:focus-visible {
    outline: none;
    box-shadow: inset 0 0 0 1px var(--accent);
  }

  .drag {
    flex: 1 0 48px;
    align-self: stretch;
  }
</style>
