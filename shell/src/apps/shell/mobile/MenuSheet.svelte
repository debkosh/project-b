<script>
  import { bookmark, isBookmarked, requestMenu, send, state as go } from '@/sdk/svelte.svelte.js';
  import { extensionsBuilt } from '@extensions';
  import Icon from './Icon.svelte';
  import Sheet from './Sheet.svelte';

  // The ⋯ sheet, shaped for a phone as Firefox's is (docs/shells.md, "The menu sheet"):
  // 1. a row of the page's actions: Back while there is somewhere to go back to (the
  //    phone's back key does it too), Forward, Bookmark, Reload or Stop, and New tab;
  // 2. a flat list: what Go's menu lifts to its top (Print…), New tab in group while the
  //    tab is in one, Find in page, and Workspaces, where new tabs open, while there are
  //    two or more;
  // 3. the places as tiles in one row: History, Bookmarks, Downloads (with how many are
  //    running) and Extensions while their flag is on;
  // 4. the update while one is ready, then Settings;
  // 5. More, the menu bar's whole tree as Go gives it, so nothing in it is out of reach.
  // Each acts and closes the sheet, but Bookmark, which shows its star filled, and
  // Workspaces, Extensions and More, which open their own.
  let { tab, group, workspace, onclose, onnewtab, onsheet, onworkspace } = $props();
  // What the sheet shows: '' its own rows, 'workspaces', or 'more', the menu bar's tree.
  let list = $state('');

  function act(msg) {
    send(msg);
    onclose();
  }

  // Go's menu for this window, which it sends as the sheet opens (docs/menus-shortcuts.md,
  // "The shell's menu"). The sheet runs Go's actions by their ids, and shows at its top
  // level what the tree has there that the sheet has no place of its own for: the entries
  // marked "shell": "top", Print… the first.
  requestMenu();
  const placed = new Set(['app.settings', 'find.open', 'history.show', 'bookmarks.show', 'downloads.show']);
  const lifted = $derived((go.menu || []).filter((item) => item.action && !item.items && !placed.has(item.action)));
  const liftedIcons = { 'page.print': 'print' };
  // An action's item in the tree, wherever it is, for whether it can run now.
  function find(items, action) {
    for (const item of items || []) {
      if (item.action === action) return item;
      const inner = find(item.items, action);
      if (inner) return inner;
    }
    return null;
  }
  const enabled = (action) => find(go.menu, action)?.enabled ?? true;
  const run = (action) => act({ t: 'run', text: action });

  // The star bookmarks the page shown, or takes its bookmark away, as Standard's does.
  const web = $derived(/^https?:/.test(tab.url || '') && !tab.page);
  const starred = $derived(isBookmarked(go, tab.url));

  // The places, each a tile running Go's action, and Extensions, which opens its sheet.
  const running = $derived(go.downloads.filter((d) => d.state === 'progress').length);
  const tiles = $derived([
    { name: 'History', icon: 'history', action: 'history.show' },
    { name: 'Bookmarks', icon: 'star', action: 'bookmarks.show' },
    { name: 'Downloads', icon: 'download', action: 'downloads.show', note: running ? `${running} running` : '' },
    ...(extensionsBuilt && go.extensions ? [{ name: 'Extensions', icon: 'extensions', sheet: 'extensions' }] : []),
  ]);

  // In More, the submenus open, by index, with a row back up, and from the top back to the
  // sheet's own rows.
  let path = $state([]);
  const menu = $derived(path.reduce((items, i) => items[i]?.items || [], go.menu || []));
  const icons = { 'find.open': 'find', 'downloads.show': 'download', 'app.settings': 'settings', 'history.show': 'history', 'bookmarks.show': 'star', 'page.print': 'print' };
  function choose(item, i) {
    if (item.items) path = [...path, i];
    else run(item.action);
  }

  const workspaces = $derived(go.workspaces || []);
  const ready = $derived(go.update?.phase === 'ready');
</script>

<Sheet label="Menu" {onclose}>
  {#if list === 'workspaces'}
    <div class="rows" role="menu" aria-label="Workspaces">
      {#each workspaces as w (w.id)}
        <button class="row" role="menuitemradio" aria-checked={workspace === w.id} onclick={() => { onworkspace(w.id); onclose(); }}>
          <span class="dot" style:background={w.color || 'var(--muted)'}></span>
          <span class="name">{w.name}</span>
          {#if workspace === w.id}<span class="tick">✓</span>{/if}
        </button>
      {/each}
    </div>
  {:else if list === 'more'}
    <div class="rows" role="menu" aria-label="Menu bar">
      <button class="row" role="menuitem" aria-label="Back to the menu" onclick={() => (path.length ? (path = path.slice(0, -1)) : (list = ''))}>
        <Icon name="back" />
        <span class="name">{path.length ? path.reduce((items, i, n) => (n === path.length - 1 ? items[i] : items[i].items), go.menu)?.title : 'More'}</span>
      </button>
      {#each menu as item, i (i)}
        {#if item.separator}
          <div class="rule" role="separator"></div>
        {:else}
          <button class="row" role={item.checked ? 'menuitemcheckbox' : 'menuitem'} aria-label={item.title} aria-checked={item.checked ? 'true' : undefined} disabled={!item.items && !item.enabled} onclick={() => choose(item, i)}>
            {#if icons[item.action]}<Icon name={icons[item.action]} />{:else}<span class="blank"></span>{/if}
            <span class="name">{item.title}</span>
            {#if item.action === 'downloads.show' && running}<span class="note">{running} running</span>{/if}
            {#if item.checked}<span class="tick">✓</span>{/if}
            {#if item.items}<Icon name="chevron" />{/if}
          </button>
        {/if}
      {/each}
    </div>
  {:else}
    <div class="actions">
      {#if tab.canGoBack}
        <button aria-label="Back" onclick={() => act({ t: 'back' })}><Icon name="back" /><span aria-hidden="true">Back</span></button>
      {/if}
      <button aria-label="Forward" disabled={!tab.canGoForward} onclick={() => act({ t: 'forward' })}><Icon name="forward" /><span aria-hidden="true">Forward</span></button>
      <button class:on={starred} aria-label="Bookmark" aria-pressed={starred} disabled={!web} onclick={() => bookmark(tab.url, tab.title, !starred)}><Icon name="star" filled={starred} /><span aria-hidden="true">{starred ? 'Bookmarked' : 'Bookmark'}</span></button>
      {#if tab.loading}
        <button aria-label="Stop" onclick={() => run('nav.stop')}><Icon name="stop" /><span aria-hidden="true">Stop</span></button>
      {:else}
        <button aria-label="Reload" disabled={!tab.url} onclick={() => act({ t: 'reload' })}><Icon name="reload" /><span aria-hidden="true">Reload</span></button>
      {/if}
      <button aria-label="New Tab" onclick={() => { onnewtab(false); onclose(); }}><Icon name="plus" /><span aria-hidden="true">New tab</span></button>
    </div>
    <div class="rows" role="menu" aria-label="Page">
      {#each lifted as item (item.action)}
        <button class="row" role="menuitem" aria-label={item.title} disabled={!item.enabled} onclick={() => run(item.action)}>
          {#if liftedIcons[item.action]}<Icon name={liftedIcons[item.action]} />{:else}<span class="blank"></span>{/if}
          <span class="name">{item.title}</span>
        </button>
      {/each}
      {#if group}
        <button class="row" role="menuitem" aria-label="New tab in group" onclick={() => { onnewtab(true); onclose(); }}>
          <Icon name="group" />
          <span class="name">New tab in group</span>
        </button>
      {/if}
      <button class="row" role="menuitem" aria-label="Find in page" disabled={!enabled('find.open')} onclick={() => run('find.open')}>
        <Icon name="find" />
        <span class="name">Find in page</span>
      </button>
      {#if workspaces.length > 1}
        <button class="row" role="menuitem" aria-label="Workspaces" onclick={() => (list = 'workspaces')}>
          <Icon name="workspace" />
          <span class="name">Workspaces</span>
          <Icon name="chevron" />
        </button>
      {/if}
    </div>
    <div class="tiles" role="menu" aria-label="Places">
      {#each tiles as t (t.name)}
        <button class="tile" role="menuitem" aria-label={t.name} aria-describedby={t.note ? `tile-note-${t.icon}` : undefined} disabled={t.action && !enabled(t.action)} onclick={() => (t.sheet ? onsheet(t.sheet) : run(t.action))}>
          <Icon name={t.icon} />
          <span class="label" aria-hidden="true">{t.name}</span>
          {#if t.note}<span class="note" id="tile-note-{t.icon}">{t.note}</span>{/if}
        </button>
      {/each}
    </div>
    <div class="rows" role="menu" aria-label="App">
      {#if ready}
        <button class="row" role="menuitem" onclick={() => act({ t: 'relaunch' })}>
          <Icon name="update" />
          <span class="name">Install update to {go.update.version}</span>
        </button>
      {/if}
      <button class="row" role="menuitem" aria-label="Settings" onclick={() => run('app.settings')}>
        <Icon name="settings" />
        <span class="name">Settings</span>
      </button>
      <button class="row quiet" role="menuitem" aria-label="More" onclick={() => { path = []; list = 'more'; }}>
        <span class="blank"></span>
        <span class="name">More</span>
        <Icon name="chevron" />
      </button>
    </div>
  {/if}
</Sheet>

<style>
  .note {
    color: var(--muted);
    font-size: 12px;
  }

  .actions {
    display: flex;
    justify-content: space-around;
    padding: 4px 8px 8px;
    border-bottom: 1px solid var(--line);
  }

  .actions button {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    min-width: 0;
    height: 56px;
    padding: 6px 0 0;
    border: 0;
    border-radius: 8px;
    background: none;
    color: var(--icon);
  }

  .actions span {
    color: var(--muted);
    font-size: 12px;
  }

  .actions button.on {
    color: var(--accent);
  }

  .actions button:disabled {
    opacity: 0.4;
  }

  .rows {
    display: flex;
    flex-direction: column;
    padding: 4px 0;
  }

  .rule {
    margin: 4px 0;
    border-top: 1px solid var(--line);
  }

  .row:disabled,
  .tile:disabled {
    opacity: 0.4;
  }

  /* An icon's room, so each row's name starts in one column. */
  .blank {
    flex: none;
    width: 20px;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 16px;
    height: 48px;
    padding: 0 20px;
    border: 0;
    background: none;
    color: var(--icon);
    text-align: left;
    font-size: 14px;
  }

  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: var(--text);
  }

  /* More is the way to everything else, not a thing to do: quiet, last. */
  .row.quiet .name {
    color: var(--muted);
  }

  /* The places, one row of tiles on the accent's tint, each at least 48px each way. */
  .tiles {
    display: grid;
    grid-auto-columns: 1fr;
    grid-auto-flow: column;
    gap: 8px;
    padding: 8px 12px;
  }

  .tile {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    min-width: 0;
    height: 72px;
    padding: 0 4px;
    border: 0;
    border-radius: 8px;
    background: var(--accent-tint);
    color: var(--icon);
  }

  .tile .label {
    max-width: 100%;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: var(--text);
    font-size: 13px;
  }

  .tile .note {
    line-height: 1;
  }

  .dot {
    width: 10px;
    height: 10px;
    margin: 0 5px;
    border-radius: 50%;
  }

  .tick {
    color: var(--accent);
  }

  button:active:not(:disabled) {
    background: var(--accent-tint);
  }

  .tile:active:not(:disabled) {
    background: var(--accent-soft);
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring-inset);
  }
</style>
