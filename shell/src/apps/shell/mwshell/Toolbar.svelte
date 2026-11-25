<script>
  import { drag, shell } from '@/sdk/svelte.svelte.js';
  import Address from './Address.svelte';
  import Icon from './Icon.svelte';

  // The one bar: the address pill filling it, the tab count, which opens the switcher, and
  // ⋯, which opens the menu; they stay while the address is edited, so the tabs are a tap
  // away from an empty new tab too. A swipe
  // sideways along it goes to the tab before or after (onswipe, -1 or 1). For one of our
  // tabs, settings, the pill is its title and a close instead, with nothing to type (rule
  // 19); the field stays, hidden, for the next web tab. The close is onclose's, as the
  // last tab's close leaves a new tab.
  let { tab, count, top, sheet, onsheet, onswipe, onedit, onclose, address = $bindable() } = $props();
  let editing = $state(false);
  let start = null;

  function down(e) {
    if (editing) return;
    start = { x: e.clientX, y: e.clientY };
  }

  function up(e) {
    if (!start) return;
    const dx = e.clientX - start.x;
    const dy = e.clientY - start.y;
    start = null;
    if (Math.abs(dx) > 48 && Math.abs(dx) > 2 * Math.abs(dy)) {
      // The press was a swipe, not a tap: the field mustn't take the keyboard for it.
      address.blur();
      onswipe(dx < 0 ? 1 : -1);
    }
  }
</script>

<div class="toolbar" role="group" aria-label="Toolbar" class:top use:shell use:drag onpointerdown={down} onpointerup={up} onpointercancel={() => (start = null)}>
  {#if tab.page}
    <div class="ours">
      <span class="title">{tab.title}</span>
      <button class="close" aria-label="Close {tab.title}" onclick={() => onclose(tab.id)}><Icon name="close" size={18} /></button>
    </div>
  {/if}
  <div class="field" class:hidden={tab.page}>
    <Address bind:this={address} {tab} {top} onsite={() => onsheet('site')} onedit={(on) => {
      editing = on;
      onedit(on);
    }} />
  </div>
  <button class="count" aria-label="Tabs" aria-haspopup="dialog" aria-expanded={sheet === 'switcher'} onclick={() => onsheet(sheet === 'switcher' ? '' : 'switcher')}>
    <span>{count > 99 ? ':D' : count}</span>
  </button>
  <button class="more" aria-label="Menu" aria-haspopup="dialog" aria-expanded={sheet === 'menu'} onclick={() => onsheet(sheet === 'menu' ? '' : 'menu')}>
    <Icon name="more" />
  </button>
</div>

<style>
  .field {
    flex: 1;
    min-width: 0;
    display: flex;
  }

  .hidden {
    display: none;
  }

  /* Our tab's title in the pill's place, as the pill looks, its close reaching 48px. */
  .ours {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    height: 40px;
    padding: 0 6px 0 14px;
    box-sizing: border-box;
    border-radius: 20px;
    background: var(--surface);
    color: var(--text);
  }

  .title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    font-size: calc(14rem / 16);
    font-weight: 500;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .close {
    flex: none;
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    margin: -4px -6px -4px 0;
    padding: 0;
    border: 0;
    border-radius: 50%;
    background: none;
    color: var(--icon);
  }

  .toolbar {
    position: relative;
    z-index: 3;
    flex: none;
    display: flex;
    align-items: center;
    gap: 4px;
    height: 48px;
    padding: 0 0 0 8px;
    box-sizing: border-box;
    background: var(--bar);
    /* The hairline drawn over the 48px row, so its 48px buttons fill it. */
    box-shadow: inset 0 1px var(--line);
    touch-action: pan-y;
  }

  .toolbar.top {
    box-shadow: inset 0 -1px var(--line);
  }

  button {
    flex: none;
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    padding: 0;
    border: 0;
    border-radius: 24px;
    background: none;
    color: var(--icon);
  }

  button:active {
    background: var(--press);
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .count span {
    display: grid;
    place-items: center;
    min-width: 20px;
    height: 20px;
    padding: 0 2px;
    box-sizing: border-box;
    border: 1.5px solid currentColor;
    border-radius: 5px;
    font-size: calc(11rem / 16);
    font-weight: 600;
    line-height: 1;
  }
</style>
