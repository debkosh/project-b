<script>
  import { find } from '@/ui/filter.js';
  import { paint } from '@/ui/hues.js';
  import { shell, state as go, tabLabel } from '@/sdk/svelte.svelte.js';
  import Favicon from './Favicon.svelte';
  import BookmarkList from './BookmarkList.svelte';
  import Icon from './Icon.svelte';
  import { gesture } from './gesture.js';
  import { groupLabel, groupOf, tabCount } from './groups.js';

  // The tab switcher, over the page: the workspaces as chips while there are two or more, which
  // choose the cards shown, and New tab; then two columns of cards, the active one ringed.
  // A tab's card: a tap shows it, its × or a swipe sideways closes it, a long press opens
  // its menu (oncard), and held and dragged onto another card it joins that one's group or
  // starts one with it (ondrop). A
  // group is one card where its first tab is, with its colour along the top and its tabs'
  // favicons, a tap on which goes to that tab; a tap elsewhere on it opens the group's
  // sheet. It stays in the document while closed, so every tab is always an element
  // (docs/shells.md, rule 4). A card shows its page as it last looked, Go's tab.thumb,
  // cropped from the top, or its favicon or letter until there is a picture.
  // A search over the workspace's tabs, by title and address, sits at the top: always there
  // with four tabs or more, behind a search button with three or fewer.
  // While bookmarksSidebar is on, Tabs and Bookmarks at the very top choose between the cards
  // and the bookmarks (docs/bookmark-folders.md, Where bookmarks show): the folders from
  // the top, a tap on a bookmark opening it in a new tab (onopen, with its address).
  let { open, groups, workspace = $bindable(), onpick, onclosetab, onnewtab, oncard, ongroup, onclosegroup, ondrop, onopen } = $props();
  let view = $state('tabs');
  const marks = $derived(!!go.prefs.bookmarksSidebar && view === 'bookmarks');

  // The pictures that didn't load, as one of a tab closed meanwhile: its favicon instead. A
  // picture is drawn only while the switcher is open, so a new one is fetched only when it
  // can be seen, not each time Go makes one.
  const broken = $state({});
  let query = $state('');
  // Whether the field was opened with the search button, on a short list.
  let searching = $state(false);
  let field = $state();
  const inWorkspace = (tab) => (tab.workspace || '') === workspace;
  const long = $derived(go.tabs.filter(inWorkspace).length > 3);
  const words = $derived(query.trim().toLowerCase());
  // Every word typed, in any order, in a tab's title or address (filter.js).
  const found = $derived(new Set(find(go.tabs, (tab) => [tabLabel(tab), tab.url || ''], words).map((tab) => tab.id)));
  const matches = (tab) => !words || found.has(tab.id);
  // Each search starts afresh: closed, the switcher forgets it.
  $effect(() => {
    if (open) return;
    query = '';
    searching = false;
    view = 'tabs';
  });

  function search() {
    searching = true;
    queueMicrotask(() => field?.focus());
  }

  function clear() {
    query = '';
    if (long) field?.focus();
    else searching = false;
  }

  // Where a card was let go: on a group's card, or one of its tabs, it joins that group;
  // on a tab's card, that tab's group, or a new one of the two.
  function dropped(id, target) {
    if (target.dataset.group) return ondrop(id, { group: target.dataset.group });
    const other = Number(target.dataset.tab);
    const g = groupOf(groups, other);
    ondrop(id, g ? { group: g.id } : { tab: other });
  }

  // What the grid shows, in Go's order: a tab, or a group at its first tab.
  const items = $derived.by(() => {
    const out = [];
    const done = new Set();
    for (const tab of go.tabs) {
      const g = groupOf(groups, tab.id);
      if (!g) out.push({ key: `t${tab.id}`, tab });
      else if (!done.has(g.id)) {
        done.add(g.id);
        out.push({ key: `g${g.id}`, group: g, tabs: g.tabs.map((id) => go.tabs.find((t) => t.id === id)).filter(Boolean) });
      }
    }
    return out;
  });
  // What the search leaves of the workspace's cards: a group's while any of its tabs match.
  const shows = (item) => (item.tab ? inWorkspace(item.tab) && matches(item.tab) : inWorkspace(item.tabs[0] || {}) && item.tabs.some(matches));
  const none = $derived(!!words && !items.some(shows));

  function menu(e, tab) {
    e.preventDefault();
    oncard(tab.id);
  }

  const workspaces = $derived(go.workspaces || []);
  const host = (url) => {
    try {
      return new URL(url).host.replace(/^www\./, '') || url;
    } catch {
      return url;
    }
  };
</script>

<div class="switcher" hidden={!open} use:shell role="dialog" aria-label="Tabs">
  {#if go.prefs.bookmarksSidebar}
    <div class="views" role="radiogroup" aria-label="Show">
      <button role="radio" aria-checked={!marks} onclick={() => (view = 'tabs')}>Tabs · {go.tabs.filter(inWorkspace).length}</button>
      <button role="radio" aria-checked={marks} onclick={() => (view = 'bookmarks')}>Bookmarks</button>
    </div>
  {/if}
  {#if marks}
    <div class="marks" role="region" aria-label="Bookmarks">
      <BookmarkList list={go.bookmarks} {onopen} />
    </div>
  {/if}
  <div class="head" hidden={marks}>
    {#if workspaces.length > 1}
      <div class="chips" role="radiogroup" aria-label="Workspace">
        {#each workspaces as w (w.id)}
          <button class="chip" role="radio" aria-checked={workspace === w.id} onclick={() => (workspace = w.id)}>
            {#if w.color}<span class="dot" style:background={paint(w.color)}></span>{/if}<span>{w.name}</span>
          </button>
        {/each}
      </div>
    {:else}
      <span class="spacer"></span>
    {/if}
    {#if !long && !searching}
      <button class="new" aria-label="Search tabs" onclick={search}><Icon name="search" /></button>
    {/if}
    <button class="new" aria-label="New Tab" onclick={onnewtab}><Icon name="plus" /></button>
  </div>
  {#if !marks && (long || searching)}
    <div class="search" role="search" aria-label="Search tabs">
      <Icon name="search" size={18} />
      <input
        bind:this={field}
        bind:value={query}
        aria-label="Search tabs"
        placeholder="Search tabs"
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
        enterkeyhint="search"
        onkeydown={(e) => {
          // Enter shows the first tab found.
          const first = e.key === 'Enter' && items.find((item) => item.tab && shows(item));
          if (first) onpick(first.tab.id);
        }}
      />
      {#if query || !long}
        <button class="clear" aria-label="Clear search" onmousedown={(e) => e.preventDefault()} onclick={clear}><Icon name="close" size={18} /></button>
      {/if}
    </div>
  {/if}
  <div class="grid" hidden={marks}>
    {#if none}<p class="none">No tabs found</p>{/if}
    {#each items as item (item.key)}
      {#if item.tab}
        {@const tab = item.tab}
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div
          class="card"
          class:active={tab.id === go.active}
          hidden={!shows(item)}
          role="tab"
          tabindex="0"
          data-tab={tab.id}
          aria-selected={tab.id === go.active}
          onclick={() => onpick(tab.id)}
          oncontextmenu={(e) => menu(e, tab)}
          use:gesture={{ onclose: () => onclosetab(tab.id), onmenu: () => oncard(tab.id), ondrop: (target) => dropped(tab.id, target) }}
          onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && onpick(tab.id)}
        >
          <div class="top">
            <span class="title">{tabLabel(tab)}</span>
            <button class="close" aria-label="Close {tabLabel(tab)}" onclick={(e) => { e.stopPropagation(); onclosetab(tab.id); }}><Icon name="close" size={16} /></button>
          </div>
          <div class="body">
            {#if open && tab.thumb && !broken[tab.thumb]}
              <img class="thumb" src={tab.thumb} alt="" draggable="false" onerror={() => (broken[tab.thumb] = true)} />
            {:else}
              <Favicon {tab} size={32} />
            {/if}
          </div>
          <span class="host">{tab.url ? host(tab.url) : 'New tab'}</span>
        </div>
      {:else}
        {@const g = item.group}
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div
          class="card group"
          class:active={g.tabs.includes(go.active)}
          hidden={!shows(item)}
          style:--group={paint(g.color)}
          role="button"
          tabindex="0"
          aria-label="Group {groupLabel(g)}"
          data-group={g.id}
          onclick={() => ongroup(g.id)}
          onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && ongroup(g.id)}
        >
          <div class="top">
            <span class="title">{groupLabel(g)}</span>
            <button class="close" aria-label="Close group {groupLabel(g)}" onclick={(e) => { e.stopPropagation(); onclosegroup(g.id); }}><Icon name="close" size={16} /></button>
          </div>
          <div class="body cluster">
            {#each item.tabs as tab, i (tab.id)}
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <span
                class="member"
                hidden={i > 3}
                role="tab"
                tabindex="-1"
                data-tab={tab.id}
                aria-label={tabLabel(tab)}
                aria-selected={tab.id === go.active}
                onclick={(e) => { e.stopPropagation(); onpick(tab.id); }}
                oncontextmenu={(e) => { e.stopPropagation(); menu(e, tab); }}
              ><Favicon {tab} size={28} /></span>
            {/each}
          </div>
          <span class="host">{g.name ? tabCount(g) : ''}</span>
        </div>
      {/if}
    {/each}
  </div>
</div>

<style>
  .switcher {
    position: absolute;
    inset: 0;
    z-index: 4;
    display: flex;
    flex-direction: column;
    background: var(--bar);
    animation: rise var(--motion) var(--ease);
  }

  .switcher[hidden] {
    display: none;
  }

  /* Tabs and Bookmarks: two halves of a pill, the one shown raised on it. */
  .views {
    flex: none;
    display: flex;
    margin: 8px 12px 4px;
    padding: 4px;
    border: 1px solid var(--line);
    border-radius: 16px;
    background: var(--accent-tint);
  }

  .views button {
    flex: 1;
    height: 48px;
    border: 0;
    border-radius: 12px;
    background: none;
    color: var(--muted);
    font-size: calc(14rem / 16);
    font-weight: 500;
  }

  .views button[aria-checked='true'] {
    background: var(--surface);
    color: var(--text);
    font-weight: 600;
    box-shadow: 0 0 0 1px var(--line);
  }

  .marks {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
    padding: 0 12px 12px;
  }

  .head[hidden],
  .grid[hidden] {
    display: none;
  }

  .head {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    height: 48px;
    padding: 0 4px 0 12px;
  }

  .chips {
    flex: 1;
    min-width: 0;
    display: flex;
    gap: 8px;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .spacer {
    flex: 1;
  }

  /* A chip is a 32px pill in a 48px target. */
  .chip {
    position: relative;
    flex: none;
    display: flex;
    align-items: center;
    gap: 6px;
    height: 48px;
    padding: 0 12px;
    border: 0;
    background: none;
    color: var(--text);
    font-size: calc(13rem / 16);
    font-weight: 500;
  }

  .chip::before {
    content: '';
    position: absolute;
    inset: 8px 0;
    border: 1px solid var(--line);
    border-radius: 16px;
    background: var(--surface);
  }

  .chip[aria-checked='true']::before {
    border-color: transparent;
    background: var(--accent-soft);
  }

  .chip > span {
    position: relative;
  }

  .dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }

  .new {
    flex: none;
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    padding: 0;
    border: 0;
    border-radius: 20px;
    background: none;
    color: var(--icon);
  }

  /* The search: a field the width of the grid, on the surface, its clear reaching 48px. */
  .search {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    height: 40px;
    margin: 0 12px 8px;
    padding: 0 0 0 14px;
    border-radius: 20px;
    background: var(--surface);
    color: var(--muted);
  }

  .search input {
    flex: 1;
    min-width: 0;
    height: 48px;
    margin: -4px 0;
    padding: 0;
    border: 0;
    outline: none;
    background: none;
    color: var(--text);
    font: inherit;
    font-size: calc(14rem / 16);
  }

  .search input::placeholder {
    color: var(--muted);
  }

  .search .clear {
    flex: none;
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    margin: -4px 0;
    padding: 0;
    border: 0;
    border-radius: 24px;
    background: none;
    color: var(--icon);
  }

  .none {
    grid-column: 1 / -1;
    margin: 24px 0;
    color: var(--muted);
    text-align: center;
  }

  .grid {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
    display: grid;
    align-content: start;
    gap: 12px;
    padding: 0 12px 12px;
    /* A tablet's is the phone's, wider. */
    grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  }

  .card {
    position: relative;
    display: flex;
    flex-direction: column;
    min-width: 0;
    box-sizing: border-box;
    border: 1px solid var(--line);
    border-radius: 12px;
    background: var(--surface);
    overflow: hidden;
    outline: none;
  }

  .card {
    touch-action: pan-y;
  }

  .card:global(.lifted) {
    z-index: 2;
    box-shadow: var(--shadow-float);
    transition: box-shadow var(--motion) var(--ease);
  }

  .card[hidden] {
    display: none;
  }

  .card.active {
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent);
  }

  .card.group.active {
    border-top-color: var(--group);
  }

  .card:focus-visible {
    box-shadow: var(--focus-ring);
  }

  .top {
    flex: none;
    display: flex;
    align-items: center;
    height: 48px;
    padding-left: 12px;
  }

  .title {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: var(--text);
    font-size: calc(13rem / 16);
    font-weight: 500;
  }

  .close {
    flex: none;
    display: grid;
    place-items: center;
    width: 48px;
    height: 48px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--icon);
  }

  .card.group {
    border-top: 4px solid var(--group);
  }

  .cluster {
    grid-template-columns: 1fr 1fr;
    gap: 4px;
    padding: 8px;
  }

  .member {
    display: grid;
    place-items: center;
    width: 100%;
    height: 100%;
  }

  .member[hidden] {
    display: none;
  }

  /* The picture's place: a phone's page, upright, 3:4, the same for every card so the
     grid stays even. The picture fills it without stretching, cropped from the top, as
     Chrome's are; without one, the favicon or letter sits in the middle. */
  .body {
    position: relative;
    aspect-ratio: 3 / 4;
    display: grid;
    place-items: center;
    margin: 0 8px;
    overflow: hidden;
    border-radius: 8px;
    background: var(--accent-tint);
  }

  .thumb {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    object-position: top;
  }

  .host {
    flex: none;
    height: 28px;
    padding: 0 12px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: var(--muted);
    font-size: calc(12rem / 16);
    line-height: calc(28rem / 16);
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  @keyframes rise {
    from { transform: translateY(24px); opacity: 0; }
  }
</style>
