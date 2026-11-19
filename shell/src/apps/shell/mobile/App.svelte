<script>
  import { closeInnermost } from '@/ui/escape.js';
  import { tick } from 'svelte';
  import { activeTab, connect, drag, focusPage, page, send, setFrame, shell, state as go } from '@/sdk/svelte.svelte.js';
  import { untrack } from 'svelte';
  import { chooseSuggestion } from '@/sdk/suggest.js';
  import { extensionsBuilt } from '@extensions';
  import BookmarkSheet from './BookmarkSheet.svelte';
  import CardMenu from './CardMenu.svelte';
  import CrashBar from './CrashBar.svelte';
  import DownloadBar from './DownloadBar.svelte';
  import FindBar from './FindBar.svelte';
  import FolderSheet from './FolderSheet.svelte';
  import Extensions from './Extensions.svelte';
  import GroupSheet from './GroupSheet.svelte';
  import LinkSheet from './LinkSheet.svelte';
  import MarksBar from './MarksBar.svelte';
  import MenuSheet from './MenuSheet.svelte';
  import SiteSheet from './SiteSheet.svelte';
  import PageDialog from './PageDialog.svelte';
  import PageNotice from './PageNotice.svelte';
  import NewTab from './NewTab.svelte';
  import Strip from './Strip.svelte';
  import Switcher from './Switcher.svelte';
  import Toolbar from './Toolbar.svelte';
  import { groupOf } from './groups.js';

  // The phone's shell: the page, and under it one bar, the address, the tab count and ⋯,
  // with the group strip over the bar while the active tab is in a group, and the bookmarks
  // bar over those while it is on (under the address bar on top). Everything else
  // is a sheet one of them opens: the switcher over the page, the rest from the bottom. The
  // page reaches the window's edges: no band.
  setFrame(0);

  const tab = $derived(activeTab(go));
  // The address bar on top, as a setting has it, rather than at the foot. The group strip
  // stays at the foot either way, as Chrome's does.
  const top = $derived(go.addressBarPosition === 'top');
  // What is open over the page: '', 'switcher', 'menu', 'extensions', 'group' (the active
  // tab's, or the one in sheetGroup), 'card' (cardTab's menu), 'link' (a long press in
  // the page), 'bookmark' (the star's panel) or 'folder' (a folder of the bookmarks bar,
  // sheetFolder).
  let sheet = $state('');
  let sheetGroup = $state('');
  let sheetFolder = $state('');
  let cardTab = $state(0);
  let askClose = $state(false);
  // The star's panel Go asks for once the menu's Bookmark has bookmarked the page
  // (docs/bookmark-folders.md), and a new key for each ask, so each starts afresh.
  let marked = $state(null);
  let asks = 0;
  $effect(() => {
    const p = go.bookmarkPanel;
    if (!p) return;
    untrack(() => {
      marked = { ...p, key: ++asks };
      sheet = 'bookmark';
    });
  });
  const markedBookmark = $derived(marked && go.bookmarks.find((b) => b.id === marked.id));
  // What a long press in the page was on, for the 'link' sheet.
  let pressed = $state(null);
  // The workspace new tabs open in, and the switcher shows: the active tab's, until the
  // user picks another.
  let workspace = $state('');
  let address = $state();
  // The tab last made active, so each change of it is seen once.
  let active = 0;
  // Whether the address is being edited, which the phone's back key ends, as it closes a
  // sheet: while either is so, the engine gives the key to the shell (backCloses).
  let editing = $state(false);
  // The shown tab's page dialog, which the back key answers no, before any sheet.
  let pageDialog;
  const asking = $derived(go.dialogs.some((d) => d.tab === go.active));
  // The shown tab's find bar, in the toolbar's place, which the back key closes.
  let findBar = $state();
  // The new tab page, while the shown tab has no page yet.
  let newTabPage = $state();
  const finding = $derived(!!go.find && go.find.tab === go.active);
  // While the phone's soft keyboard is up the bars under the page step aside, so a dialog
  // or a field in the page has the room down to the keyboard; they stay while the shell's
  // own field has it, the address or find (docs/design-system.md, On a phone).
  let keyboard = $state(false);
  const aside = $derived(keyboard && !editing && !finding);
  $effect(() => send({ t: 'backCloses', on: !!sheet || editing || asking || finding }));

  // The phone's back key: what is open over a sheet closes, as the Custom colour sheet
  // (escape.js), the page's dialog is answered no, the sheet open closes, or the editing
  // ends.
  function backKey() {
    if (closeInnermost()) return;
    if (asking) pageDialog.cancel();
    else if (sheet) sheet = '';
    else if (finding) findBar.close();
    else if (editing) {
      address.blur();
      focusPage();
    }
  }

  // The bars hide while the page scrolls down and come back when it scrolls up, for another
  // tab, the address, a sheet or a page that starts loading (docs/shells.md, Bars that hide
  // on scroll). The page's room changes only while the bars cover it, so nothing but the page
  // ever shows where they were, and the page is resized once each way:
  // - Going, the page takes their room first, under them (over), and once it has drawn
  //   there they slide off it (away).
  // - Coming back, they slide in over the page, and once they are in the page gives their
  //   room back.
  // A change of mind mid-slide turns the slide round where it is. shown is what was last
  // asked for, which the window carries as data-bars for a test; over and away follow it.
  let over = $state(false);
  let away = $state(false);
  let shown = $state(true);
  // Each ask's number, so a slide that ends after another ask leaves the room alone.
  let asked = 0;
  // The bars' heights, for the margins that lay them over the page; and --covered, how much
  // of the page's foot they cover while over it and in place, which what floats at the
  // foot keeps clear of.
  let headHeight = $state(0);
  let stripHeight = $state(0);
  let footHeight = $state(0);
  // Whether the page has just taken the bars' room or given it back. Resizing the page
  // under a finger is a scroll of its own to content: the page moves under the finger, or
  // its end comes closer, and content says the direction changed. Taken for the person's,
  // that turned the bars back and forth near the top of a page. A direction while
  // settling, settle ms from a resize, is the resize's, and is let go; it also keeps a
  // finger's small wobble from turning the bars at once. The window carries data-settling
  // meanwhile, for a test to wait on.
  let settling = $state(false);
  let settled = 0;
  const settle = 300;

  function room(give) {
    if (over === give) return;
    over = give;
    settling = true;
    clearTimeout(settled);
    settled = setTimeout(() => (settling = false), settle);
  }

  // A few frames: the page's new room goes to Go in the next one, and the page draws into
  // it in the ones after, still under the bars.
  function frames(n, then) {
    requestAnimationFrame(() => (n > 1 ? frames(n - 1, then) : then()));
  }

  async function bars(show) {
    if (show === shown) return;
    shown = show;
    const ask = ++asked;
    if (show) {
      // In over the page from wherever they are, and the room back once they're in: when
      // their slides end, or at once with none, as when they never left, under reduced
      // motion, or while they aren't drawn.
      away = false;
      await tick();
      const slides = [...document.querySelectorAll('.bars')].flatMap((bar) => bar.getAnimations());
      await Promise.allSettled(slides.map((slide) => slide.finished));
      if (ask === asked) room(false);
    } else {
      room(true);
      frames(3, () => ask === asked && (away = true));
    }
  }

  // A page that starts loading in the active tab brings the bars back, as Chrome's do.
  let loading = '';
  $effect(() => {
    const now = tab.loading ? tab.url : '';
    if (now && now !== loading) bars(true);
    loading = now;
  });

  function scrolled(m) {
    const editing = document.activeElement?.getAttribute('aria-label') === 'Address';
    if (settling) return;
    if (m.tab === go.active && !sheet && !editing && !finding) bars(m.direction !== 'down');
  }

  $effect(() => {
    if (sheet) bars(true);
  });

  // Space for the native window buttons, which a phone hasn't: a desktop window, as on
  // the mock, keeps the page from under them.
  // A page in full screen, the shown tab's: the shell draws nothing over it, its bars and
  // notices gone and their room the page's, and a sheet open or the address being edited
  // ends as it starts. Go ends it on the back key.
  // A first start's welcome has the screen so too, with the system's bars kept
  // (docs/onboarding.md).
  const full = $derived((go.pageFullScreen !== 0 && go.pageFullScreen === go.active) || (go.welcome !== 0 && go.welcome === go.active));
  $effect(() => {
    if (!full) return;
    sheet = '';
    address?.blur();
  });
  const clear = $derived(go.controls?.h > 0 ? go.controls.y + go.controls.h : 0);

  // Go's tab groups (docs/groups.md): the active tab's, and the one whose sheet is open.
  const group = $derived(groupOf(go.groups, go.active));
  const tabsOf = (g) => g.tabs.map((id) => go.tabs.find((t) => t.id === id)).filter(Boolean);
  const shownGroup = $derived(go.groups.find((g) => g.id === sheetGroup) || null);

  // The bookmarks bar, while its pref is on (docs/bookmark-folders.md, Where bookmarks
  // show): one of the bars, under the address bar on top, else in the strip's part, over
  // the group strip, so it hides with them and the page's room counts it. A bookmark in it,
  // or in a folder's sheet, loads in the shown tab.
  const marksBar = $derived(!!go.prefs.bookmarksBar);
  function openMark(url) {
    sheet = '';
    send({ t: 'navigate', text: url });
    focusPage();
  }
  function openFolder(id) {
    sheetFolder = id;
    sheet = 'folder';
  }

  // A tab that becomes active with a page wants the page. One with nothing in it shows the
  // new tab page, with the keyboard down so it covers none of it; a tap on the field types.
  function activated() {
    if (go.active === active) return;
    active = go.active;
    bars(true);
    workspace = tab.workspace || '';
    address.blur();
    if (tab.url !== '') focusPage();
  }

  // A new tab, or with a tab's id, one in that tab's group, starting one with it when it
  // has none.
  function newTab(groupWith = 0) {
    sheet = '';
    send(groupWith ? { t: 'newTab', groupWith } : { t: 'newTab', workspace });
  }

  // Open all, on a bookmark folder of the new tab page: the first bookmark in this tab, the
  // rest behind it, in a group with it named after the folder, as Chrome's Open all in a
  // new tab group. Go starts the group with the second tab and says nothing of its id, so
  // the name follows once the group shows; a tab already in a group adds them to it as
  // they are.
  let naming = $state(null);
  function openAll(name, urls) {
    const [first, ...rest] = urls;
    naming = rest.length && !group ? { tab: go.active, name } : null;
    send({ t: 'navigate', text: first });
    for (const url of rest) send({ t: 'newTab', groupWith: go.active, text: url, background: true });
  }
  $effect(() => {
    const made = naming && groupOf(go.groups, naming.tab);
    if (!made) return;
    send({ t: 'editGroup', group: made.id, name: naming.name });
    naming = null;
  });

  // A closed tab, from the new tab page: Go reopens it and shows it, and the new tab it was
  // chosen on then goes, unless it has started a page of its own.
  let leaving = $state(0);
  function reopen(row) {
    leaving = go.active;
    chooseSuggestion(row);
  }
  $effect(() => {
    if (!leaving || go.active === leaving) return;
    if (go.tabs.some((t) => t.id === leaving && !t.url && !t.loading)) send({ t: 'closeTab', id: leaving });
    leaving = 0;
  });

  function pick(id) {
    sheet = '';
    if (id !== go.active) send({ t: 'activateTab', id });
    else focusPage();
  }

  // Closing the active tab of a group goes to its neighbour in the group, as the strip
  // shows it. No tabs is a new tab: closing the last ones leaves one, in the workspace new
  // tabs open in, opened first so the window is never without a tab (docs/daily-driver.md,
  // A new tab). Back on it closes it, and with it the app.
  function closeTabs(ids) {
    if (ids.length >= go.tabs.length) {
      sheet = '';
      send({ t: 'newTab', workspace });
    } else if (ids.includes(go.active) && group) {
      const left = group.tabs.filter((id) => !ids.includes(id));
      const at = group.tabs.indexOf(go.active);
      const next = left[Math.min(at, left.length - 1)];
      if (next) send({ t: 'activateTab', id: next });
    }
    for (const id of ids) send({ t: 'closeTab', id });
  }

  // The tab before or after the active one: in its group while it is in one.
  function swipe(by) {
    const ids = group ? group.tabs : go.tabs.map((t) => t.id);
    const next = ids[ids.indexOf(go.active) + by];
    if (next) send({ t: 'activateTab', id: next });
  }

  function onsheet(name) {
    // A picture of the page as it is now for its card, asked for before the switcher
    // covers it (docs/shells.md, captureTab).
    if (name === 'switcher') {
      send({ t: 'captureTab' });
      workspace = tab.workspace || '';
    }
    if (name === 'group') sheetGroup = group?.id || '';
    askClose = false;
    sheet = name;
  }

  // The download bar, which draws Go's download news.
  let downloadBar = $state();

  function openGroup(id, ask = false) {
    sheetGroup = id;
    askClose = ask;
    sheet = 'group';
  }

  connect({
    snapshot: activated,
    tabs: activated,
    // The find key, or Find in page from the menu: the bar shows with the keyboard in it.
    focusFind: () => {
      sheet = '';
      bars(true);
      queueMicrotask(() => findBar?.focus());
    },
    focusAddressBar: () => {
      sheet = '';
      bars(true);
      address.focus();
    },
    scroll: scrolled,
    // Go's right-click menu: the sheet opens on one for the shown tab. Go drops it when
    // the tab changes, and says so with null.
    contextMenu: ({ contextMenu: m }) => {
      if (!m || m.tab !== go.active) return;
      // The same menu again brings its image's picture: the sheet takes it where it is.
      if (m.id === pressed?.id && sheet !== 'link') return;
      pressed = m;
      sheet = 'link';
    },
    backKey,
    downloadNews: (m) => downloadBar?.news(m),
    keyboard: ({ on }) => (keyboard = on),
    suggestions: (m) => {
      address.suggestions(m);
      newTabPage?.suggestions(m);
    },
  });
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && sheet && ((sheet = ''), e.preventDefault())} />

<div class="window" data-settling={settling || undefined} data-bars={shown ? 'shown' : 'hidden'} style:--head="{headHeight}px" style:--strip="{stripHeight}px" style:--foot="{footHeight}px" style:--covered="{over && !away ? stripHeight + footHeight : 0}px">
  {#if clear && !full}
    <div class="clear" use:shell use:drag style:height="{clear}px"></div>
  {/if}
  {#if top}
    <div class="bars head" class:away class:over class:full bind:offsetHeight={headHeight}>
      <div class="tool">
        <Toolbar {tab} top count={go.tabs.length} {sheet} {onsheet} onswipe={swipe} onedit={(on) => (editing = on)} onclose={(id) => closeTabs([id])} bind:address />
        {#if finding}<FindBar bind:this={findBar} />{/if}
      </div>
      {#if marksBar}<MarksBar list={go.bookmarks} folder={go.prefs.bookmarksBarFolder} top onopen={openMark} onfolder={openFolder} />{/if}
    </div>
  {/if}
  <div class="stage">
    <main use:page={go.active}>
      {#if !full && tab.notice}
        <PageNotice {tab} />
      {:else if !full && !tab.url && !tab.loading}
        {#key tab.id}<NewTab bind:this={newTabPage} onopenall={openAll} onreopen={reopen} />{/key}
      {/if}
      <!-- A download as it happens, over the foot of the page; never over a sheet, nor over
           the Downloads page, which shows it already. -->
      <DownloadBar bind:this={downloadBar} hidden={!!sheet || full || tab.page === 'downloads'} away={aside} />
    </main>
    {#if group || (marksBar && !top)}
      <div class="bars strip" class:away class:over class:full bind:offsetHeight={stripHeight}>
        {#if marksBar && !top}<MarksBar list={go.bookmarks} folder={go.prefs.bookmarksBarFolder} onopen={openMark} onfolder={openFolder} />{/if}
        {#if group}<Strip {group} tabs={tabsOf(group)} active={go.active} onsheet={() => onsheet('group')} onpick={pick} onclosetab={(id) => closeTabs([id])} onnewtab={() => newTab(go.active)} />{/if}
      </div>
    {/if}
    <Switcher
      open={sheet === 'switcher'}
      groups={go.groups}
      bind:workspace
      onpick={pick}
      onclosetab={(id) => closeTabs([id])}
      onnewtab={() => newTab()}
      onopen={(url) => {
        sheet = '';
        send({ t: 'newTab', workspace, text: url });
      }}
      oncard={(id) => {
        cardTab = id;
        sheet = 'card';
      }}
      ongroup={(id) => openGroup(id)}
      ondrop={(id, to) => send(to.group ? { t: 'addToGroup', id, group: to.group } : { t: 'createGroup', ids: [to.tab, id] })}
      onclosegroup={(id) => openGroup(id, true)}
    />
  </div>
  <div class="bars foot" class:away class:over class:full class:aside bind:offsetHeight={footHeight}>
    <CrashBar />
    {#if !top}
      <div class="tool">
        <Toolbar {tab} count={go.tabs.length} {sheet} {onsheet} onswipe={swipe} onedit={(on) => (editing = on)} onclose={(id) => closeTabs([id])} bind:address />
        {#if finding}<FindBar bind:this={findBar} />{/if}
      </div>
    {/if}
  </div>
  {#if sheet === 'menu'}
    <MenuSheet {tab} {group} {workspace} onclose={() => (sheet = '')} onnewtab={(inGroup) => newTab(inGroup ? go.active : 0)} {onsheet} onworkspace={(id) => (workspace = id)} />
  {:else if sheet === 'site' && /^https?:/.test(tab.url || '')}
    <SiteSheet {tab} onclose={() => (sheet = '')} />
  {:else if sheet === 'bookmark' && markedBookmark}
    {#key marked.key}
      <BookmarkSheet bookmark={markedBookmark} list={go.bookmarks} added={marked.new} onclose={() => (sheet = '')} />
    {/key}
  {:else if sheet === 'group' && shownGroup}
    {#key shownGroup.id}
      <GroupSheet
        group={shownGroup}
        tabs={tabsOf(shownGroup)}
        ask={askClose}
        onclose={() => (sheet = '')}
        onchange={(fields) => send({ t: 'editGroup', group: shownGroup.id, ...fields })}
        onpick={pick}
        onclosetab={(id) => closeTabs([id])}
        onnewtab={() => newTab(shownGroup.tabs.at(-1))}
        onungroup={() => {
          send({ t: 'ungroup', group: shownGroup.id });
          sheet = '';
        }}
        onclosegroup={() => {
          sheet = '';
          if (shownGroup.tabs.length >= go.tabs.length) send({ t: 'newTab', workspace });
          send({ t: 'closeGroup', group: shownGroup.id });
        }}
      />
    {/key}
  {:else if sheet === 'folder' && go.bookmarks.some((e) => e.id === sheetFolder)}
    {#key sheetFolder}
      <FolderSheet list={go.bookmarks} folder={sheetFolder} onclose={() => (sheet = '')} onopen={openMark} />
    {/key}
  {:else if sheet === 'link' && pressed}
    <LinkSheet
      menu={pressed}
      onclose={() => (sheet = '')}
      onopen={(m) => {
        sheet = '';
        send({ t: 'newTab', ...m });
      }}
    />
  {:else if sheet === 'card' && go.tabs.some((t) => t.id === cardTab)}
    {@const t = go.tabs.find((x) => x.id === cardTab)}
    {@const own = groupOf(go.groups, cardTab)}
    <CardMenu
      tab={t}
      group={own}
      groups={go.groups.filter((g) => g !== own && (tabsOf(g)[0]?.workspace || '') === (t.workspace || ''))}
      onclose={() => (sheet = '')}
      onadd={(id) => {
        send({ t: 'addToGroup', id: cardTab, group: id });
        sheet = 'switcher';
      }}
      onnewgroup={() => newTab(cardTab)}
      onremove={() => {
        send({ t: 'removeFromGroup', id: cardTab });
        sheet = 'switcher';
      }}
      onclosetab={() => {
        closeTabs([cardTab]);
        if (sheet === 'card') sheet = 'switcher';
      }}
    />
  {/if}
  {#if extensionsBuilt && go.extensions}
    <Extensions extensions={go.extensions} open={sheet === 'extensions'} onclose={() => (sheet = '')} />
  {/if}
</div>
<PageDialog bind:this={pageDialog} />

<style>
  .window {
    position: relative;
    display: flex;
    flex-direction: column;
    height: 100%;
    color: var(--text);
    font-size: 14px;
    -webkit-user-select: none;
    user-select: none;
  }

  .clear {
    flex: none;
    background: var(--bar);
  }

  .stage {
    position: relative;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }

  /* The page's area is a layer of its own: what lies over the page (a notice, the find bar, a
     page's dialog) stays under the shell's bars and the menus and panels they open
     (design-system.md, Layers). */
  main {
    position: relative;
    isolation: isolate;
    flex: 1;
    min-height: 0;
  }

  .tool {
    position: relative;
  }

  .bars {
    flex: none;
    display: flex;
    flex-direction: column;
    transition: transform var(--motion) var(--ease);
  }

  /* Over the page, which has their room: each bar's margin takes its own height back, so
     the page's part runs on under it, the strip staying above the toolbar. Above the
     page's notices; under the switcher and the sheets. */
  .bars.over {
    position: relative;
    z-index: 3;
  }

  .head.over {
    margin-bottom: calc(-1 * var(--head));
  }

  .strip.over {
    margin-top: calc(-1 * (var(--strip) + var(--foot)));
    margin-bottom: var(--foot);
  }

  .foot.over {
    margin-top: calc(-1 * var(--foot));
  }

  /* Off the screen: each one by its own height and those of the bars beyond it. */
  .head.away {
    transform: translateY(-100%);
  }

  .strip.away {
    transform: translateY(calc(100% + var(--foot)));
  }

  .foot.aside {
    display: none;
  }

  .foot.away {
    transform: translateY(100%);
  }

  /* Under a page in full screen: not there at all, to the eye or a screen reader. */
  .bars.full {
    display: none;
  }


  :global(button) {
    font-family: inherit;
    -webkit-tap-highlight-color: transparent;
  }
</style>
