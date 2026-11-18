<script>
  import { bookmarksUnder, inFolder, path } from '@/ui/bookmarks.js';
  import Icon from './Icon.svelte';
  import Mark from './Mark.svelte';
  import { folderMark, tints } from '@/ui/tint.js';

  // A bookmark folder as a list, root to start with (docs/bookmark-folders.md): the path to
  // it from root, each folder on it a tap away; then its folders, each with how many
  // bookmarks it holds, a tap going into one; then its bookmarks, a tap on one opening it
  // (onopen, with its address).
  let { list, root = '', onopen } = $props();
  // svelte-ignore state_referenced_locally
  let at = $state(root);
  const trail = $derived.by(() => {
    const all = path(list, at);
    return all.slice(all.findIndex((f) => f.id === root));
  });
  const entries = $derived(inFolder(list, at));

  function host(url) {
    try {
      return new URL(url).host.replace(/^www\./, '') || url;
    } catch {
      return url;
    }
  }
</script>

<nav class="trail" aria-label="Path">
  {#each trail as f, i (f.id)}
    {#if i}<Icon name="chevron" size={14} />{/if}
    <button class:here={f.id === at} aria-current={f.id === at ? 'location' : undefined} onclick={() => (at = f.id)}>{f.title}</button>
  {/each}
</nav>
{#if entries.length}
  <ul class="entries" aria-label={trail.at(-1).title}>
    {#each entries as e (e.id)}
      <li>
        {#if e.kind === 'folder'}
          <button class="row" onclick={() => (at = e.id)}>
            <span class="icon" style:color={($tints, folderMark(e.id))}><Icon name="folder" /></span>
            <span class="title">{e.title}</span>
            <span class="count">{bookmarksUnder(list, e.id)}</span>
            <Icon name="chevron" size={16} />
          </button>
        {:else}
          <button class="row" onclick={() => onopen(e.url)}>
            <Mark url={e.url} title={e.title} size={24} />
            <span class="lines">
              <span class="title">{e.title || e.url}</span>
              <span class="host">{host(e.url)}</span>
            </span>
          </button>
        {/if}
      </li>
    {/each}
  </ul>
{:else}
  <p class="empty">This folder is empty.</p>
{/if}

<style>
  .trail {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    color: var(--muted);
  }

  .trail button {
    min-width: 48px;
    height: 48px;
    padding: 0 4px;
    border: 0;
    background: none;
    color: var(--muted);
    font-size: 14px;
  }

  .trail button.here {
    color: var(--text);
    font-weight: 600;
  }

  .entries {
    margin: 0;
    padding: 0;
    list-style: none;
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: 16px;
    background: var(--surface);
  }

  li + li {
    border-top: 1px solid var(--line);
  }

  .row {
    display: flex;
    align-items: center;
    gap: 14px;
    width: 100%;
    min-height: 56px;
    padding: 0 16px;
    border: 0;
    background: none;
    color: var(--icon);
    text-align: left;
  }

  .row:active {
    background: var(--press);
  }

  .icon {
    display: grid;
    place-items: center;
    width: 24px;
  }

  .lines {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .title,
  .host {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .title {
    color: var(--text);
    font-size: 15px;
  }

  .row > .title {
    flex: 1;
    min-width: 0;
  }

  .host,
  .count {
    color: var(--muted);
    font-size: 12px;
  }

  .empty {
    margin: 16px 4px;
    color: var(--muted);
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring-inset);
  }
</style>
