<script>
  import { send, shell, state as go } from '@/sdk/svelte.svelte.js';
  import { chooseSuggestion } from '@/sdk/suggest.js';
  import { inFolder, path } from '@/ui/bookmarks.js';
  import Icon from './Icon.svelte';
  import Mark from './Mark.svelte';
  import { nextQuery } from './queries.js';

  // Over a new tab, which has no page yet (docs/daily-driver.md, A new tab): the sites
  // visited most as tiles, named by their host, then while bookmarksNewTab is on the top
  // bookmark folder as tiles, its folders first (docs/bookmark-folders.md, Where bookmarks
  // show), then All bookmarks and History. The sites are Go's list for nothing typed. A tap
  // loads a site or a bookmark in this tab; a long press on a site's tile shows each tile's
  // ×, which takes that site out of history. A folder's tile shows the first four of its
  // bookmarks, and a tap opens it in place, in a card under its row: what it holds, as
  // tiles, and Open all (onopenall). The keyboard stays down: a tap on the address field
  // starts typing.
  let { onopenall } = $props();
  const sites = $state({ id: 0, items: [] });
  let editing = $state(false);

  function ask() {
    sites.id = nextQuery();
    send({ t: 'suggest', id: sites.id, text: '', kinds: ['history'] });
  }
  ask();

  // Go's answer to its ask.
  export function suggestions(m) {
    if (m.id === sites.id) sites.items = m.items;
  }

  function forget(row) {
    send({ t: 'forgetSite', url: row.url });
    ask();
  }

  // Four tiles to a row, as the sites have.
  const perRow = 4;
  const marks = $derived(go.prefs.bookmarksNewTab ? inFolder(go.bookmarks, '') : []);
  // The top folder's folder whose card is open, and the folder the card shows: that one, or
  // a folder in it the person went into. The card goes when its folder does.
  let open = $state('');
  let shown = $state('');
  const at = $derived(marks.findIndex((e) => e.id === open));
  // The card follows the last tile of the open folder's row.
  const cardAfter = $derived(at < 0 ? -1 : Math.min(marks.length - 1, at - (at % perRow) + perRow - 1));
  const card = $derived(at < 0 ? [] : inFolder(go.bookmarks, shown));
  const cardBookmarks = $derived(card.filter((e) => e.kind === 'bookmark'));
  const cardTitle = $derived(at < 0 ? '' : path(go.bookmarks, shown).at(-1).title);

  function toggle(id) {
    open = open === id ? '' : id;
    shown = open;
  }

  function choose(entry) {
    if (entry.kind === 'folder') shown = entry.id;
    else send({ t: 'navigate', text: entry.url });
  }
</script>

{#snippet tile(entry, onclick)}
  <button class="tile" class:open={entry.id === open} aria-expanded={entry.kind === 'folder' ? entry.id === open : undefined} {onclick}>
    {#if entry.kind === 'folder'}
      {@const some = inFolder(go.bookmarks, entry.id).filter((e) => e.kind === 'bookmark').slice(0, 4)}
      <span class="folder" aria-hidden="true">
        {#each some as b (b.id)}<Mark url={b.url} title={b.title} size={20} />{:else}<Icon name="folder" />{/each}
      </span>
    {:else}
      <Mark url={entry.url} title={entry.title} />
    {/if}
    <span class="name">{entry.title || entry.url}</span>
  </button>
{/snippet}

<div class="new-tab" use:shell role="region" aria-label="New tab" onclick={(e) => e.target === e.currentTarget && (editing = false)}>
  {#if sites.items.length}
    <h2>Frequently visited</h2>
    <ul class="tiles">
      {#each sites.items as row (row.url)}
        <li>
          <button class="tile" oncontextmenu={(e) => { e.preventDefault(); editing = true; }} onclick={() => (editing ? (editing = false) : chooseSuggestion(row))}>
            <Mark url={row.url} title={row.detail} />
            <span class="name">{row.detail}</span>
          </button>
          {#if editing}
            <button class="forget" aria-label="Remove {row.detail} from history" onclick={() => forget(row)}><Icon name="close" size={14} /></button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
  {#if marks.length}
    <h2>Bookmarks</h2>
    <ul class="tiles" aria-label="Bookmarks">
      {#each marks as entry, i (entry.id)}
        <li>{@render tile(entry, () => (entry.kind === 'folder' ? toggle(entry.id) : choose(entry)))}</li>
        {#if i === cardAfter}
          <li class="card" role="group" aria-label={cardTitle} style:--col={at % perRow}>
            <div class="head">
              {#if shown !== open}
                <button class="back" aria-label="Back" onclick={() => (shown = path(go.bookmarks, shown).at(-2).id)}><Icon name="back" /></button>
              {/if}
              <span class="title">{cardTitle}</span>
              <span class="count">· {cardBookmarks.length}</span>
              {#if cardBookmarks.length}
                <button class="open-all" onclick={() => onopenall(cardTitle, cardBookmarks.map((b) => b.url))}>Open all</button>
              {/if}
            </div>
            {#if card.length}
              <ul class="tiles">
                {#each card as entry (entry.id)}
                  <li>{@render tile(entry, () => choose(entry))}</li>
                {/each}
              </ul>
            {:else}
              <p class="empty">This folder is empty.</p>
            {/if}
          </li>
        {/if}
      {/each}
    </ul>
  {/if}
  <div class="rows">
    <button class="row link" onclick={() => send({ t: 'run', text: 'bookmarks.show' })}><Icon name="star" /><span class="name">All bookmarks</span></button>
    <button class="row link" onclick={() => send({ t: 'run', text: 'history.show' })}><Icon name="history" /><span class="name">History</span></button>
  </div>
</div>

<style>
  .new-tab {
    position: absolute;
    inset: 0;
    z-index: 2;
    overflow-y: auto;
    scrollbar-width: none;
    padding: 24px 16px;
    box-sizing: border-box;
    background: var(--bar);
    color: var(--text);
  }

  h2 {
    margin: 0 0 12px 4px;
    color: var(--muted);
    font-size: 13px;
    font-weight: 600;
  }

  .tiles {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 16px 8px;
    margin: 0 0 28px;
    padding: 0;
    list-style: none;
  }

  .tiles li {
    position: relative;
  }

  .tile {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    width: 100%;
    min-height: 48px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--text);
    font-size: 12px;
  }

  /* A folder: the first four of its bookmarks, two by two, on a tile the size of a site's. */
  .folder {
    display: grid;
    grid-template-columns: repeat(2, 20px);
    place-content: center;
    gap: 4px;
    width: 56px;
    height: 56px;
    box-sizing: border-box;
    border: 1px solid var(--line);
    border-radius: 16px;
    background: var(--surface);
    color: var(--icon);
  }

  .folder > :global(svg) {
    grid-column: 1 / -1;
  }

  .tile.open .folder {
    border: 2px solid var(--text);
  }

  /* The open folder's card, the width of the grid under its folder's row, a notch on its
     edge under the folder. */
  .card {
    position: relative;
    grid-column: 1 / -1;
    padding: 4px 8px 16px;
    border: 1px solid var(--line);
    border-radius: 16px;
    background: var(--surface);
  }

  .card::before {
    content: '';
    position: absolute;
    top: -7px;
    left: calc((var(--col) + 0.5) * 25% - 6px);
    width: 12px;
    height: 12px;
    border-top: 1px solid var(--line);
    border-left: 1px solid var(--line);
    background: var(--surface);
    transform: rotate(45deg);
  }

  .card .head {
    display: flex;
    align-items: center;
    gap: 6px;
    min-height: 48px;
    padding-left: 8px;
  }

  .card .back {
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    margin-left: -12px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--icon);
  }

  .card .title {
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-size: 15px;
    font-weight: 600;
  }

  .card .count {
    flex: 1;
    color: var(--muted);
  }

  .open-all {
    flex: none;
    height: 48px;
    padding: 0 8px;
    border: 0;
    background: none;
    color: var(--text);
    font-size: 14px;
    font-weight: 600;
  }

  .card .tiles {
    margin: 8px 0 0;
  }

  .empty {
    margin: 8px;
    color: var(--muted);
  }

  .tile .name {
    max-width: 76px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .forget {
    position: absolute;
    top: -6px;
    right: 4px;
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    padding: 0;
    border: 1px solid var(--line);
    border-radius: 50%;
    background: var(--surface);
    color: var(--text);
  }

  /* 48px of room to press, as every control has, around the badge. */
  .forget::before {
    content: '';
    position: absolute;
    inset: -12px;
  }

  .rows {
    display: flex;
    flex-direction: column;
    margin-bottom: 20px;
    overflow: hidden;
    border-radius: 16px;
    background: var(--surface);
  }

  .row {
    display: flex;
    align-items: center;
    gap: 12px;
    height: 52px;
    padding: 0 16px;
    border: 0;
    border-top: 1px solid var(--line);
    background: none;
    color: var(--text);
    text-align: left;
    font-size: 14px;
  }

  .row:first-child {
    border-top: 0;
  }

  .row .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .row.link {
    color: var(--icon);
    font-weight: 500;
  }

  .row.link .name {
    color: var(--icon);
  }
</style>
