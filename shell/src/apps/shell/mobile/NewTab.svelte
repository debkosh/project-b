<script>
  import { send, shell } from '@/sdk/svelte.svelte.js';
  import { chooseSuggestion } from '@/sdk/suggest.js';
  import Icon from './Icon.svelte';
  import { nextQuery } from './queries.js';

  // Over a new tab, which has no page yet (docs/daily-driver.md, A new tab): the sites
  // visited most as tiles, named by their host, then the newest bookmarks, then All bookmarks and History. All
  // of it is Go's list for nothing typed, asked for once per kind. A tap loads it in this
  // tab; a long press on a tile shows each tile's ×, which takes that site out of history.
  // The keyboard stays down: a tap on the address field starts typing.
  const sites = $state({ id: 0, items: [] });
  const marks = $state({ id: 0, items: [] });
  let editing = $state(false);

  function ask() {
    sites.id = nextQuery();
    send({ t: 'suggest', id: sites.id, text: '', kinds: ['history'] });
    marks.id = nextQuery();
    send({ t: 'suggest', id: marks.id, text: '', kinds: ['bookmark'] });
  }
  ask();

  // Go's answer to one of its own asks.
  export function suggestions(m) {
    if (m.id === sites.id) sites.items = m.items;
    else if (m.id === marks.id) marks.items = m.items.slice(0, 3);
  }

  function host(url) {
    try {
      return new URL(url).host.replace(/^www\./, '');
    } catch {
      return url;
    }
  }

  // A tile's letter and colour, from its site, the same each time.
  function tile(url) {
    const h = host(url);
    let n = 0;
    for (const c of h) n = (n * 31 + c.charCodeAt(0)) % 360;
    return { letter: (h[0] || '?').toUpperCase(), hue: n };
  }

  function forget(row) {
    send({ t: 'forgetSite', url: row.url });
    ask();
  }
</script>

<div class="new-tab" use:shell role="region" aria-label="New tab" onclick={(e) => e.target === e.currentTarget && (editing = false)}>
  {#if sites.items.length}
    <h2>Frequently visited</h2>
    <ul class="tiles">
      {#each sites.items as row (row.url)}
        {@const t = tile(row.url)}
        <li>
          <button class="tile" oncontextmenu={(e) => { e.preventDefault(); editing = true; }} onclick={() => (editing ? (editing = false) : chooseSuggestion(row))}>
            <span class="mark" style:--hue={t.hue} aria-hidden="true">{t.letter}</span>
            <span class="name">{row.detail}</span>
          </button>
          {#if editing}
            <button class="forget" aria-label="Remove {row.detail} from history" onclick={() => forget(row)}><Icon name="close" size={14} /></button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
  {#if marks.items.length}
    <h2>Bookmarks</h2>
    <div class="rows">
      {#each marks.items as row (row.url)}
        <button class="row" onclick={() => chooseSuggestion(row)}>
          <span class="name">{row.title}</span>
          <span class="detail">{row.detail}</span>
        </button>
      {/each}
    </div>
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

  .tile .mark {
    display: grid;
    place-items: center;
    width: 56px;
    height: 56px;
    border-radius: 16px;
    background: hsl(var(--hue) 45% 42%);
    color: #fff;
    font-size: 20px;
    font-weight: 600;
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

  .row .detail {
    color: var(--muted);
    font-size: 12px;
  }

  .row.link {
    color: var(--icon);
    font-weight: 500;
  }

  .row.link .name {
    color: var(--icon);
  }
</style>
