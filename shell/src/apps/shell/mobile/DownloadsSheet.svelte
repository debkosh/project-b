<script>
  import { clearDownloads, downloadAction, state as go } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';
  import Sheet from './Sheet.svelte';

  // The profile's downloads, newest first (docs/daily-driver.md): each by its file name, with
  // how far it has got or how it ended. A running one can be paused, resumed or cancelled;
  // a finished one opens, or shows in the phone's Downloads, where it is kept. Clear takes
  // the finished ones off the list, leaving their files. The menu's Downloads opens it.
  let { onclose } = $props();

  // A size as a phone gives it: 1000 to a kilobyte, one decimal place under ten.
  function size(n) {
    if (n < 1000) return `${n} B`;
    for (const [unit, by] of [['KB', 1e3], ['MB', 1e6], ['GB', 1e9], ['TB', 1e12]]) {
      const v = n / by;
      if (v < 1000 || unit === 'TB') return `${v < 10 ? +v.toFixed(1) : Math.round(v)} ${unit}`;
    }
  }

  function status(d) {
    switch (d.state) {
      case 'progress':
        return (d.paused ? 'Paused, ' : '') + (d.total > 0 ? `${size(d.received)} of ${size(d.total)}` : size(d.received));
      case 'done':
        return d.total > 0 ? size(d.total) : 'Done';
      case 'cancelled':
        return 'Cancelled';
      default:
        return 'Failed';
    }
  }

  const finished = $derived(go.downloads.some((d) => d.state !== 'progress'));
  // The list emptying, as Clear can leave it, closes the sheet.
  $effect(() => {
    if (!go.downloads.length) onclose();
  });
</script>

<Sheet label="Downloads" {onclose}>
  <div class="head">
    <h2>Downloads</h2>
    {#if finished}<button class="text-button clear" onclick={clearDownloads}>Clear</button>{/if}
  </div>
  <ul>
    {#each go.downloads as d (d.id)}
      <li aria-label={d.name}>
        <span class="mark" class:failed={d.state === 'failed' || d.state === 'cancelled'}><Icon name="file" /></span>
        <span class="lines">
          <span class="name">{d.name}</span>
          <span class="status">{status(d)}</span>
          {#if d.state === 'progress'}
            <span class="bar" class:unknown={d.total <= 0}><i style:width={d.total > 0 ? `${(d.received / d.total) * 100}%` : ''}></i></span>
          {/if}
        </span>
        {#if d.state === 'progress'}
          {#if d.paused}
            <button aria-label="Resume" onclick={() => downloadAction(d.id, 'resume')}><Icon name="play" /></button>
          {:else}
            <button aria-label="Pause" onclick={() => downloadAction(d.id, 'pause')}><Icon name="pause" /></button>
          {/if}
          <button aria-label="Cancel" onclick={() => downloadAction(d.id, 'cancel')}><Icon name="close" /></button>
        {:else if d.state === 'done'}
          <button class="text-button" onclick={() => downloadAction(d.id, 'open')}>Open</button>
          <button aria-label="Show in Downloads" onclick={() => downloadAction(d.id, 'reveal')}><Icon name="download" /></button>
        {:else}
          <button aria-label="Remove {d.name}" onclick={() => downloadAction(d.id, 'remove')}><Icon name="close" /></button>
        {/if}
      </li>
    {/each}
  </ul>
</Sheet>

<style>
  .head {
    display: flex;
    align-items: center;
    padding: 0 8px 0 20px;
  }

  h2 {
    flex: 1;
    margin: 0;
    font-size: 15px;
    font-weight: 600;
  }

  .clear {
    height: 48px;
    padding: 0 16px;
    border: 0;
    background: none;
    color: var(--accent);
    font-weight: 600;
  }

  ul {
    margin: 0;
    padding: 0 8px 8px 20px;
    list-style: none;
  }

  li {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 64px;
  }

  li + li {
    border-top: 1px solid var(--line);
  }

  .mark {
    display: grid;
    color: var(--icon);
  }

  .mark.failed {
    color: var(--muted);
  }

  .lines {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .name {
    overflow: hidden;
    font-size: 15px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .status {
    color: var(--muted);
    font-size: 13px;
  }

  .bar {
    position: relative;
    height: 3px;
    margin-top: 4px;
    overflow: hidden;
    border-radius: 2px;
    background: var(--line);
  }

  .bar i {
    position: absolute;
    inset: 0 auto 0 0;
    background: var(--accent);
  }

  .bar.unknown i {
    width: 30%;
    animation: slide 1.2s linear infinite;
  }

  @keyframes slide {
    from { transform: translateX(-100%); }
    to { transform: translateX(340%); }
  }

  li button {
    flex: none;
    display: grid;
    place-items: center;
    min-width: 48px;
    height: 48px;
    padding: 0;
    border: 0;
    border-radius: 24px;
    background: none;
    color: var(--icon);
  }

  li .text-button {
    padding: 0 16px;
    border: 0;
    color: var(--accent);
    font-weight: 600;
  }

  @media (prefers-reduced-motion: reduce) {
    .bar.unknown i { animation: none; }
  }
</style>
