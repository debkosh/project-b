<script>
  import { untrack } from 'svelte';
  import { downloadAction, shell, state as go } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';

  // A download seen as it happens (docs/shells.md, "A download you can see"): a small bar
  // over the foot of the page, above the toolbar. "Downloading big.zip" for a few seconds
  // when one starts while the app is showing; "big.zip downloaded" with Open when it ends
  // while the bar or the app is showing, Open doing what the Downloads page's does; and
  // "Couldn't download big.zip" with Retry when it fails. A file the app kept itself, a
  // saved crash report, says what it is instead: "Crash report saved to Downloads", with
  // Open. A tap anywhere else or a swipe down on it puts it away. hidden is while a sheet
  // is open, which it must not cover, or the Downloads page shows, which says it all
  // already: it goes then, and says nothing until that ends, so it never comes back over
  // what the person went on to. The find bar is in the toolbar's place, under it.
  let { hidden } = $props();

  // What the bar says: the download's id, name and state as they were, or null.
  let said = $state(null);
  let bar = $state();
  let timer = 0;
  let start = null;
  // Each download's state as last seen, so each change is told once; null until Go's
  // snapshot is in, whose downloads were there before and are not told.
  let seen = null;

  // How long it stays: a start is a moment's news, an end with something to do longer.
  const lasts = { progress: 4000, done: 6000, failed: 6000 };

  // Why one failed, in the words the desktop's lists use (docs/daily-driver.md).
  const why = { connection: 'the connection stopped', server: 'the server refused it', disk: 'not enough disk space' };

  function say(d) {
    clearTimeout(timer);
    said = { id: d.id, name: d.name, state: d.state, from: d.from, reason: d.reason };
    timer = setTimeout(() => (said = null), lasts[d.state]);
  }

  function dismiss() {
    clearTimeout(timer);
    said = null;
  }

  $effect(() => {
    if (hidden) dismiss();
  });

  $effect(() => {
    const list = go.downloads;
    if (!go.loaded) return;
    if (!seen) {
      seen = new Map(list.map((d) => [d.id, d.state]));
      return;
    }
    const showing = document.visibilityState === 'visible';
    const quiet = untrack(() => hidden);
    // Oldest first, so the newest change is the one said.
    for (const d of [...list].reverse()) {
      const was = seen.get(d.id);
      seen.set(d.id, d.state);
      if (was === d.state) continue;
      const shown = said?.id === d.id;
      if (quiet) continue;
      if (d.state === 'progress' && showing) say(d);
      else if ((d.state === 'done' || d.state === 'failed') && (shown || showing)) say(d);
      else if (shown) dismiss();
    }
  });

  function act(action) {
    downloadAction(said.id, action);
    dismiss();
  }

  function down(e) {
    start = e.clientY;
    e.currentTarget.setPointerCapture(e.pointerId);
  }

  function up(e) {
    if (start !== null && e.clientY - start > 24) dismiss();
    start = null;
  }
</script>

<svelte:window onpointerdowncapture={(e) => said && !bar?.contains(e.target) && dismiss()} />

<div class="slot" aria-live="polite">
  {#if said}
    <div class="bar" bind:this={bar} use:shell role="group" aria-label="Download" onpointerdown={down} onpointerup={up} onpointercancel={() => (start = null)}>
      <span class="mark" class:failed={said.state === 'failed'}><Icon name={said.state === 'failed' ? 'warn' : 'download'} /></span>
      <span class="words">
        {#if said.state === 'progress'}Downloading {said.name}{:else if said.state === 'done' && said.from}{said.from} saved to Downloads{:else if said.state === 'done'}{said.name} downloaded{:else}Couldn't download {said.name}{why[said.reason] ? `: ${why[said.reason]}` : ''}{/if}
      </span>
      {#if said.state === 'done'}
        <button onclick={() => act('open')}>Open</button>
      {:else if said.state === 'progress'}
        <!-- Every progress has Retry and Cancel (docs/ux-guide.md); Pause is on the
             Downloads page, which has the room. -->
        <button onclick={() => act('cancel')}>Cancel</button>
        <button onclick={() => act('retry')}>Retry</button>
      {:else if said.state === 'failed'}
        <button onclick={() => act('retry')}>Retry</button>
      {/if}
    </div>
  {/if}
</div>

<style>
  /* Above the bars while they lie over the page (--covered, App.svelte). */
  .slot {
    position: absolute;
    left: 8px;
    right: 8px;
    bottom: calc(8px + var(--covered, 0px));
    z-index: 3;
  }

  /* Floating: the surface, a hairline, radius 8 and the floating shadow. */
  .bar {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 48px;
    padding: 0 4px 0 16px;
    box-sizing: border-box;
    border: 1px solid var(--line);
    border-radius: 8px;
    background: var(--surface);
    box-shadow: var(--shadow-float);
    color: var(--text);
    touch-action: none;
    animation: rise var(--motion) var(--ease);
  }

  .mark {
    flex: none;
    display: grid;
    color: var(--icon);
  }

  .mark.failed {
    color: var(--danger);
  }

  /* Two lines at most, so a failure's reason isn't cut off; a long name still ends in an
     ellipsis. */
  .words {
    flex: 1;
    min-width: 0;
    display: -webkit-box;
    overflow: hidden;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    padding: 6px 0;
    font-size: 14px;
    line-height: 18px;
  }

  button {
    flex: none;
    height: 48px;
    padding: 0 16px;
    border: 0;
    border-radius: 8px;
    background: none;
    color: var(--icon);
    font: inherit;
    font-weight: 600;
  }

  button:active {
    background: var(--accent-tint);
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring-inset);
  }

  @keyframes rise {
    from { transform: translateY(16px); opacity: 0; }
  }
</style>
