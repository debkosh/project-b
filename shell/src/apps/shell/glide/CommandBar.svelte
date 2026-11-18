<script>
  import { activeTab, chooseSuggestion, send, shell, state as go, suggest, tabLabel } from '@/sdk/svelte.svelte.js';
  import Favicon from './Favicon.svelte';
  import Icon from './Icon.svelte';
  import * as g from './glide.svelte.js';

  // The Command Bar: one field for where to go and what to do. It finds open tabs in every
  // Space, actions by name (a leading / lists only them), the history, and the address or
  // search typed: Go's one list of suggestions. Opened on the address (Cmd+L, the address bar, a new empty tab), Enter
  // takes the active tab there; opened empty (Open Command Bar), it opens a new tab in the
  // Space. Cmd+Enter opens it in a split beside the active tab instead. Up and Down choose,
  // Escape or a press outside closes.
  let { close } = $props();
  const bar = g.ui.bar;
  let input;
  let text = $state(bar.text);
  let chosen = $state(0);
  // Go's list for what is typed: rows of the address, the search, tabs, history and actions.
  let list = $state([]);
  let asked = 0;

  const query = $derived(text.trim());
  const sections = { address: 'Open', search: 'Search', tab: 'Tabs', history: 'History', action: 'Actions' };
  const kinds = { address: 'go', search: 'go', tab: 'tab', history: 'visit', action: 'action' };

  // With nothing typed, the other tabs of this Space; once something is, Go's one list,
  // which every field that takes an address draws (docs/daily-driver.md).
  const rows = $derived.by(() => {
    if (query === '' || query === '/') {
      return g
        .tabsIn(g.spaceId())
        .filter((t) => t.id !== go.active)
        .slice(0, 6)
        .map((t) => ({ kind: 'tab', section: 'Tabs', tab: t, label: tabLabel(t), detail: g.hostOf(t.url), row: { kind: 'tab', tab: t.id } }));
    }
    // Glide's own order: words look among the tabs, history and actions first, so the
    // search Go puts first goes last when anything else matched.
    const ordered = list.length > 1 && list[0].kind === 'search' ? [...list.slice(1), list[0]] : list;
    return ordered.map((r) => ({
      kind: kinds[r.kind],
      section: sections[r.kind],
      row: r,
      url: r.url,
      // A tab of another window isn't among this window's: it shows without its icon.
      tab: r.kind === 'tab' ? go.tabs.find((t) => t.id === r.tab) || { id: r.tab, url: r.url, title: r.title } : null,
      label: r.kind === 'search' ? `Search for "${r.title}"` : r.title,
      detail: r.kind === 'search' || r.kind === 'address' ? '' : r.detail,
    }));
  });

  $effect(() => {
    if (chosen >= rows.length) chosen = Math.max(0, rows.length - 1);
  });

  function oninput() {
    bar.typed = true;
    chosen = 0;
    list = [];
    // A leading / lists only actions.
    if (query.startsWith('/')) {
      if (query.length > 1) suggest(++asked, query.slice(1), ['action']);
    } else if (query) suggest(++asked, query);
  }

  // Go runs the row, except where Glide places it itself: beside the active tab in a split,
  // or as a new tab in the Space shown.
  function run(row, aside) {
    if (!row) {
      // Enter before Go's list has come: what was typed goes as it is, and Go reads it as
      // an address or a search.
      if (!query || query.startsWith('/')) return;
      if (aside && activeTab(go).id) g.openInSplit(query);
      else if (bar.mode === 'address' && activeTab(go).id) send({ t: 'navigate', text: query, typed: true });
      else send({ t: 'newTab', workspace: g.spaceId(), text: query, typed: true });
      close();
      return;
    }
    const r = row.row;
    if (r.kind === 'tab' || r.kind === 'action') chooseSuggestion(r);
    else if (aside && activeTab(go).id) g.openInSplit(r.url);
    else if (bar.mode === 'address' && activeTab(go).id) chooseSuggestion(r);
    else send({ t: 'newTab', workspace: g.spaceId(), text: r.url, typed: r.typed });
    close();
  }

  function onkeydown(e) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      if (rows.length) chosen = (chosen + (e.key === 'ArrowDown' ? 1 : -1) + rows.length) % rows.length;
    } else if (e.key === 'Enter') {
      e.preventDefault();
      run(rows[chosen], e.metaKey || e.ctrlKey);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      close();
    }
  }

  // What the window asks of the bar: to take the keyboard, with what it holds selected,
  // and the history Go found.
  export function focus() {
    input?.focus();
    input?.select();
  }

  export function suggestions(m) {
    if (m.id === asked) list = m.items || [];
  }

  const icons = { tab: 'tab', action: 'bolt', visit: 'clock', go: 'globe' };
</script>

<!-- Over the pages only: the sidebar stays in reach, and a press there closes the bar. -->
<div class="scrim" use:shell role="presentation" style:left={g.win.hidden ? 0 : g.win.width + 4 + 'px'} style:top={g.win.hidden ? '36px' : 0} onpointerdown={(e) => e.target === e.currentTarget && close()}>
  <div class="bar" role="dialog" aria-label="Command Bar">
    <div class="input">
      <Icon name="search" size={18} />
      <input
        bind:this={input}
        bind:value={text}
        role="combobox"
        aria-label="Address"
        aria-expanded={rows.length > 0}
        aria-controls="rows"
        aria-autocomplete="list"
        aria-activedescendant={rows.length ? `row-${chosen}` : undefined}
        placeholder={bar.mode === 'new' ? 'Search, enter an address, or find a tab' : 'Search or enter an address'}
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
        {oninput}
        {onkeydown}
      />
    </div>
    {#if rows.length}
      <ul id="rows" role="listbox" aria-label="Results" onmousedown={(e) => e.preventDefault()}>
        {#each rows as row, i (i)}
          {#if i === 0 || rows[i - 1].section !== row.section}
            <li class="section" role="presentation">{row.section}</li>
          {/if}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <li id="row-{i}" role="option" aria-selected={i === chosen} class:chosen={i === chosen} onmouseenter={() => (chosen = i)} onclick={(e) => run(row, e.metaKey || e.ctrlKey)}>
            {#if row.kind === 'tab'}<Favicon tab={row.tab} />{:else}<span class="icon"><Icon name={icons[row.kind]} /></span>{/if}
            <span class="label">{row.label}</span>
            {#if row.detail}<span class="detail">{row.detail}</span>{/if}
          </li>
        {/each}
      </ul>
    {/if}
    <div class="hints"><span>↩ Go</span><span>⌘↩ Split View</span><span>/ Actions</span><span>⎋ Close</span></div>
  </div>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    background: var(--g-scrim);
  }

  .bar {
    position: absolute;
    top: 112px;
    left: 50%;
    width: min(640px, calc(100% - 32px));
    transform: translateX(-50%);
    border: 1px solid var(--line);
    border-radius: 8px;
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--g-float-shadow);
    overflow: hidden;
  }

  .input {
    display: flex;
    align-items: center;
    gap: 12px;
    height: 52px;
    padding: 0 16px;
    color: var(--muted);
  }

  input {
    flex: 1;
    min-width: 0;
    height: 100%;
    padding: 0;
    border: 0;
    outline: none;
    background: none;
    color: var(--text);
    font: inherit;
    font-size: 18px;
  }

  ul {
    margin: 0;
    padding: 4px 8px 8px;
    list-style: none;
    border-top: 1px solid var(--line);
  }

  .section {
    padding: 8px 8px 4px;
    color: var(--muted);
    font-size: 12px;
    font-weight: 500;
  }

  [role='option'] {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 36px;
    padding: 0 8px;
    border-radius: 6px;
    white-space: nowrap;
  }

  .chosen {
    background: var(--accent-tint);
  }

  .icon {
    display: grid;
    place-items: center;
    width: 16px;
    color: var(--icon);
  }

  .label {
    flex: none;
    max-width: 60%;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .detail {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--muted);
    font-size: 12px;
    text-align: right;
  }

  .hints {
    display: flex;
    gap: 16px;
    height: 32px;
    align-items: center;
    padding: 0 16px;
    border-top: 1px solid var(--line);
    color: var(--muted);
    font-size: 12px;
  }
</style>
