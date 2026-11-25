<script>
  import { tick } from 'svelte';
  import { chooseSuggestion, focusPage, send, shell, state as app, typedFirst } from '@/sdk/svelte.svelte.js';
  import { inFolder } from '@/ui/bookmarks.js';
  import Icon from './Icon.svelte';
  import Mark from './Mark.svelte';
  import { popupsWord } from '@/sdk/sites.js';
  import { nextQuery } from './queries.js';

  // The address pill. At rest it shows the host and the page's lock; a tap turns it into
  // the field, with the whole address selected (docs/daily-driver.md, What the address field
  // suggests). One layer: while the field is focused either its list covers the page down to
  // the keyboard, or nothing covers it. With nothing typed over a new tab nothing does, as
  // the new tab page is the list; over a page it is the page's own row, with Copy, Share and
  // Edit, and while bookmarksCommandBox is on the bookmarks as one row of tiles. Typing asks
  // Go for its list: Enter's row first, completed in the field, then up to two open tabs,
  // then the pages and bookmarks by their visits, the search last, as many rows as fit, five
  // under three letters. The list lies away from the field towards the page, the first row
  // next to it: a tap on a row goes there, its ↖ puts it in the field. Enter takes Enter's
  // row only while the list answers what is typed now, else what was typed. Escape or a
  // press in the page puts the address back.
  let { tab, top, onedit, onsite } = $props();
  let input;
  let editing = $state(false);
  // What the field shows, and what was typed of it: they differ by the completion, which
  // shows selected after the caret.
  let text = $state('');
  let typed = '';
  let inline = '';
  // A delete says the person wants what they typed, so the next answer completes nothing.
  let deleting = false;
  // Whether the keyboard composes words: it holds what it composes, and a completion in the
  // field, put in under it or already there when a word begins, is typed over with the word
  // again, Android 10's Gboard doubling a letter ("llocalhost", "/ttblocked"). Once it has
  // composed in an edit, the field completes nothing more until the edit ends, and a
  // completion on show when a word begins goes; a keyboard that types letters as keys
  // completes as before.
  let composes = false;
  let items = $state([]);
  // The id of the last suggest sent: only its answer is listed.
  let asked = 0;
  // The text the last suggest was for, and the text the list on screen answers, which the
  // list carries as data-answers: until Go's answer comes, it is an earlier text's.
  let askedFor = '';
  let answers = $state('');
  // Whether nothing is typed: the field as it was tapped, or emptied.
  let blank = $state(true);
  // The room between the field and the keyboard, which the list covers.
  let room = $state(0);

  // While bookmarksCommandBox is on, with nothing typed on a page the bookmarks show as one
  // row of tiles, sideways for more, every bookmark of every folder; folders live on the new
  // tab page and in All bookmarks (docs/bookmark-folders.md, Where bookmarks show).
  const marksOn = $derived(!!app.prefs.bookmarksCommandBox && app.bookmarks.some((e) => e.kind === 'bookmark'));
  const under = (id) => inFolder(app.bookmarks, id).flatMap((e) => (e.kind === 'folder' ? under(e.id) : [e]));
  const marks = $derived(marksOn ? under('') : []);
  const onPage = $derived(editing && blank && !!tab.url);
  const open = $derived(editing && (onPage || (!blank && items.length > 0)));

  const host = $derived.by(() => {
    if (!tab.url) return '';
    try {
      const u = new URL(tab.url);
      return u.protocol === 'http:' || u.protocol === 'https:' ? u.host.replace(/^www\./, '') : tab.url;
    } catch {
      return tab.url;
    }
  });
  const bareURL = (u) => u.replace(/^https?:\/\/(www\.)?/, '');
  // What went wrong with the page, said at the pill's end.
  const status = $derived(tab.blocked ? 'Blocked' : tab.crashed ? 'Crashed' : tab.error ? 'Failed' : '');
  // The lock only for a page that shows: one that failed, crashed or was blocked has the
  // warning, beside the pill's word for what went wrong.
  const secure = $derived(status ? 'warn' : tab.url.startsWith('https:') ? 'lock' : tab.url.startsWith('http:') ? 'warn' : '');
  // A page that reached the tab over a connection that isn't secure, an http page or a
  // certificate passed with Open anyway, says so in words at the pill's start, as Chrome
  // does; the words are part of the site's mark, which opens the site's sheet.
  const insecure = $derived(!status && !!tab.connection && tab.connection !== 'secure');
  // Each row's icon says its kind, so a typed list has no headings.
  const kindIcons = { tab: 'tab', history: 'history', bookmark: 'star', search: 'search', address: 'globe', closed: 'closed' };
  // The list from the field outwards: under a field on top, over one at the bottom with
  // the first row nearest it.
  const shown = $derived(top ? items : [...items].reverse());

  $effect(() => onedit(editing));

  // The room from the field to the keyboard, or to the window's top over a bar at the foot.
  function measure() {
    const r = input?.closest('.address')?.getBoundingClientRect();
    if (!r) return;
    room = Math.max(0, top ? window.innerHeight - r.bottom - 4 : r.top - 4);
  }
  $effect(() => {
    if (!editing) return;
    measure();
    window.addEventListener('resize', measure);
    return () => window.removeEventListener('resize', measure);
  });

  // How many rows Go is asked for: five under three letters; from three as many 48px rows as
  // fit, at least five and at most eight.
  function rowsFor(t) {
    if ([...t.trim()].length < 3) return 5;
    return Math.min(8, Math.max(5, Math.floor(room / 48)));
  }

  function begin() {
    if (editing) return;
    editing = true;
    text = tab.url;
    typed = '';
    inline = '';
    blank = true;
    items = [];
    // After the value is in, so the whole of it is selected.
    queueMicrotask(() => input.select());
  }

  function end() {
    editing = false;
    composes = false;
    items = [];
    asked = 0;
    inline = '';
  }

  function ask() {
    askedFor = typed;
    const kinds = ['address', 'search', 'tab', 'history', ...(app.prefs.bookmarksCommandBox ? ['bookmark'] : [])];
    send({ t: 'suggest', id: (asked = nextQuery()), text: typed, kinds, rows: rowsFor(typed) });
  }

  function oninput(e) {
    deleting = e?.inputType?.startsWith('delete') ?? false;
    inline = '';
    typed = text;
    if (text.trim() === '') {
      blank = true;
      items = [];
      asked = 0;
      return;
    }
    blank = false;
    ask();
  }

  // showInline puts the completion after what was typed, selected, so the next key replaces
  // it, Backspace takes it away, and Enter goes where the field shows.
  async function showInline(rest) {
    inline = rest;
    text = typed + rest;
    await tick();
    input?.setSelectionRange(typed.length, text.length, 'backward');
  }

  // A key that matches the completion's next letter keeps the rest of it on show, rather than
  // dropping it until Go answers again.
  function onbeforeinput(e) {
    if (!inline || e.inputType !== 'insertText' || !e.data) return;
    if (input.selectionStart !== typed.length || input.selectionEnd !== text.length) return;
    if (!inline.toLowerCase().startsWith(e.data.toLowerCase())) return;
    e.preventDefault();
    typed += e.data;
    showInline(inline.slice(e.data.length));
    ask();
  }

  // Enter: with nothing typed, the address again; otherwise Enter's row, the first, while
  // the field shows its completion or the list answers what is typed now; else, as before
  // any list comes, what was typed, so a fast typist's Enter never opens a row of the list
  // for an earlier text.
  function enter() {
    if (blank) return go(text);
    const completes = !!inline && text === typed + inline;
    const row = completes || answers === typed ? items[0] : undefined;
    if (row) choose(row);
    else go(typed);
  }

  function go(value) {
    if (value.trim() === '') return;
    send({ t: 'navigate', text: value.trim(), typed: true });
    leave();
  }

  // A row of the list: Go shows its tab, or loads its address or search in this tab.
  function choose(row) {
    chooseSuggestion(row);
    leave();
  }

  function leave() {
    input.blur();
    focusPage();
  }

  function fill(value) {
    text = value;
    input.focus();
    oninput();
  }

  // The page's row: Copy and Share hand its address on, Edit puts the caret at the end of it.
  function copy() {
    send({ t: 'copyText', text: tab.url });
    leave();
  }

  function share() {
    send({ t: 'shareText', text: tab.url });
    leave();
  }

  function edit() {
    input.focus();
    input.setSelectionRange(text.length, text.length);
  }

  // What the shell asks of the field.
  export function focus() {
    input.focus();
    begin();
  }

  export function blur() {
    input.blur();
  }

  export function suggestions(m) {
    if (m.id !== asked || !editing) return;
    let list = m.items;
    const first = list[0];
    if (first?.inline && deleting) list = typedFirst(list, askedFor);
    items = list;
    answers = askedFor;
    // Completed only while the field holds what was typed with the caret at its end, or with
    // the completion still selected after it.
    if (!input) return;
    const end = input.selectionEnd === text.length;
    const plain = text === typed && input.selectionStart === input.selectionEnd && end;
    const still = !!inline && text === typed + inline && input.selectionStart === typed.length && end;
    if (list[0]?.inline && !composes && (plain || still)) showInline(list[0].inline);
    else if (still) {
      inline = '';
      text = typed;
    }
  }
</script>

<div class="address" class:editing>
  {#if !editing && insecure}
    <button type="button" class="mark insecure" aria-label="Not secure · Site settings" onclick={onsite}><Icon name="warn" size={16} /><span aria-hidden="true">Not secure</span></button>
  {:else if !editing && secure}
    <button type="button" class="mark" aria-label="Site settings" onclick={onsite}><Icon name={secure} size={16} /></button>
  {/if}
  <input
    bind:this={input}
    value={editing ? text : host}
    {onbeforeinput}
    oncompositionstart={() => {
      composes = true;
      if (inline) {
        inline = '';
        text = typed;
      }
    }}
    oninput={(e) => {
      text = e.currentTarget.value;
      oninput(e);
    }}
    aria-label="Address"
    placeholder="Search or type an address"
    autocomplete="off"
    autocapitalize="off"
    spellcheck="false"
    enterkeyhint="go"
    inputmode="url"
    onfocus={begin}
    onblur={() => {
      end();
      // A press in the page takes the window's keyboard, not the field's: let it go, so
      // the field doesn't take it back when the shell next has the keyboard.
      setTimeout(() => !document.hasFocus() && document.activeElement === input && input.blur());
    }}
    onkeydown={(e) => {
      if (e.key === 'Enter') enter();
      else if (e.key === 'Escape') leave();
    }}
  />
  {#if !editing && tab.popupsBlocked}
    <!-- The popups Links to other sites closed on this page: a tap opens the site's sheet. -->
    <button type="button" class="popups" aria-label={popupsWord(tab.popupsBlocked)} onclick={onsite}>
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <rect x="2" y="3.5" width="9" height="8" rx="1.5" />
        <path d="M5 1.75h7.75a1.5 1.5 0 0 1 1.5 1.5V9M2.5 13.5l11-11" />
      </svg>
      {tab.popupsBlocked}
    </button>
  {/if}
  {#if !editing && status}
    <span class="status">{status}</span>
  {/if}
  {#if editing && text}
    <!-- A press here mustn't take the keyboard from the field. -->
    <button class="clear" aria-label="Clear" onmousedown={(e) => e.preventDefault()} onclick={() => fill('')}><Icon name="close" size={18} /></button>
  {/if}
  {#if !editing && tab.loading}
    <span class="progress" style:width="{Math.max(tab.progress, 0.05) * 100}%"></span>
  {/if}
  {#if open}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="drop" class:below={top} style:height="{room}px" use:shell onmousedown={(e) => e.preventDefault()}>
      {#if onPage}
        {#if top}{@render page()}{/if}
        {#if marks.length}{@render bookmarks()}{/if}
        {#if !top}{@render page()}{/if}
      {:else}
        <ul class="suggestions" class:below={top} aria-label="Suggestions" data-answers={answers}>
          {#each shown as item (`${item.kind}:${item.tab || item.url}`)}
            <li>
              <button class="go" onclick={() => choose(item)}>
                <Icon name={kindIcons[item.kind] || 'globe'} />
                <span class="lines">
                  <span class="title">{item.title || item.url}</span>
                  <span class="url">{item.kind === 'search' || item.kind === 'address' ? item.detail : bareURL(item.url)}</span>
                </span>
                {#if item.kind === 'tab'}<span class="switch-tab">Switch to tab</span>{/if}
              </button>
              {#if item.kind !== 'tab'}
                <button class="fill" aria-label="Edit {item.kind === 'search' ? item.title : item.url}" onclick={() => fill(item.kind === 'search' ? item.title : item.url)}><Icon name="fill" /></button>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</div>

{#snippet page()}
  <div class="this-page" role="group" aria-label="This page">
    <Mark url={tab.url} title={tab.title} size={32} />
    <span class="lines">
      <span class="title">{tab.title || host}</span>
      <span class="url">{bareURL(tab.url)}</span>
    </span>
    <button class="fill" aria-label="Copy address" onclick={copy}><Icon name="copy" /></button>
    <button class="fill" aria-label="Share" onclick={share}><Icon name="share" /></button>
    <button class="fill" aria-label="Edit address" onclick={edit}><Icon name="edit" /></button>
  </div>
{/snippet}

{#snippet bookmarks()}
  <div class="marks" role="region" aria-label="Bookmarks">
    <p class="label" aria-hidden="true">Bookmarks</p>
    <ul class="tiles" aria-label="Bookmarks">
      {#each marks as b (b.id)}
        <li><button class="tile" onclick={() => go(b.url)}><Mark url={b.url} title={b.title} size={48} /><span class="name">{b.title || b.url}</span></button></li>
      {/each}
    </ul>
  </div>
{/snippet}

<style>
  .address {
    position: relative;
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    height: 40px;
    padding: 0 6px 0 14px;
    gap: 6px;
    box-sizing: border-box;
    border-radius: 20px;
    background: var(--surface);
    color: var(--text);
  }

  .address.editing {
    box-shadow: inset 0 0 0 1.5px var(--accent);
  }

  /* The site's mark opens its sheet: a 48px target, past the pill's own height. */
  .mark {
    display: grid;
    place-items: center;
    flex: none;
    width: 48px;
    height: 48px;
    margin: -4px -12px -4px -16px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--icon);
  }

  /* Not secure: the warning and the words in --danger, a hairline parting them from the
     host. The button grows to hold the words, keeping its 48px height. */
  .mark.insecure {
    display: flex;
    gap: 4px;
    width: auto;
    margin-right: 0;
    padding: 0 8px 0 16px;
    color: var(--danger);
    font: inherit;
    font-size: calc(12 * var(--capped-rem) / 16);
    font-weight: 500;
    white-space: nowrap;
  }

  .mark.insecure span {
    padding-right: 8px;
    border-right: 1px solid var(--line);
    line-height: calc(16 * var(--capped-rem) / 16);
  }

  /* The field and its Clear reach 48px, the toolbar's height, past the 40px pill. */
  input {
    flex: 1;
    min-width: 0;
    height: 48px;
    margin: -4px 0;
    padding: 0;
    border: 0;
    outline: none;
    background: none;
    color: inherit;
    font-family: inherit;
    font-size: calc(14 * var(--capped-rem) / 16);
    font-weight: 500;
    text-overflow: ellipsis;
  }

  .editing input {
    font-weight: 400;
  }

  input::placeholder {
    color: var(--muted);
    font-weight: 400;
  }

  /* A 48px target past the pill's height, as the site's mark is. */
  .popups {
    flex: none;
    display: flex;
    align-items: center;
    gap: 3px;
    min-width: 48px;
    height: 48px;
    margin: -4px 0;
    padding: 0 8px;
    border: 0;
    background: none;
    color: var(--icon);
    font: inherit;
    font-size: calc(13 * var(--capped-rem) / 16);
    font-weight: 600;
  }

  .status {
    flex: none;
    padding: 2px 8px;
    border: 1px solid var(--danger);
    border-radius: 10px;
    color: var(--danger);
    font-size: calc(12 * var(--capped-rem) / 16);
  }

  .clear {
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

  .progress {
    position: absolute;
    left: 14px;
    bottom: 0;
    max-width: calc(100% - 28px);
    height: 2px;
    border-radius: 1px;
    background: var(--accent);
    transition: width var(--motion) var(--ease);
  }

  /* What lies away from the field towards the page, covering it down to the keyboard: the
     suggestions, or the page's row and the bookmarks. Over a bar at the foot its rows sit
     nearest the field. */
  .drop {
    position: absolute;
    /* The toolbar's width: the 8px before the pill, and the count and ⋯ after it, 48px each
       with a 4px gap before each. */
    left: -8px;
    right: -104px;
    bottom: calc(100% + 4px);
    z-index: 5;
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
    box-sizing: border-box;
    background: var(--surface);
    border-top: 1px solid var(--line);
  }

  .drop.below {
    top: calc(100% + 4px);
    bottom: auto;
    justify-content: flex-start;
    border-top: 0;
  }

  .suggestions {
    flex: 0 1 auto;
    min-height: 0;
    margin: 0;
    padding: 0;
    list-style: none;
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  /* The bookmarks: one row of tiles, scrolling sideways. */
  .marks {
    flex: none;
    padding: 4px 0 8px;
  }

  .label {
    display: flex;
    align-items: center;
    height: 28px;
    margin: 0;
    padding: 0 16px;
    color: var(--muted);
    font-size: calc(13 * var(--capped-rem) / 16);
    font-weight: 500;
  }

  .tiles {
    display: flex;
    margin: 0;
    padding: 0 8px;
    overflow-x: auto;
    scrollbar-width: none;
    list-style: none;
  }

  .marks .tiles li {
    flex: none;
    height: auto;
    border: 0;
  }

  .tile {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    width: 76px;
    padding: 4px 0 0;
    border: 0;
    background: none;
    color: var(--text);
    font-size: calc(12 * var(--capped-rem) / 16);
  }

  .tile .name {
    max-width: 72px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  li {
    display: flex;
    align-items: center;
    height: 48px;
    border-bottom: 1px solid var(--line);
  }

  .below li {
    border-bottom: 0;
    border-top: 1px solid var(--line);
  }

  li:last-child {
    border-bottom: 0;
  }

  .go {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 14px;
    height: 100%;
    padding: 0 0 0 16px;
    border: 0;
    background: none;
    color: var(--icon);
    text-align: left;
  }

  .lines {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .title,
  .url {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .title {
    color: var(--text);
    font-size: calc(14 * var(--capped-rem) / 16);
  }

  .url {
    color: var(--muted);
    font-size: calc(12 * var(--capped-rem) / 16);
  }

  .fill {
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

  button:focus-visible {
    box-shadow: var(--focus-ring-inset);
  }

  /* The page's own row, with nothing typed: its mark, title and address, then Copy, Share
     and Edit, on the bar's colour, as the field it belongs to. */
  .this-page {
    flex: none;
    display: flex;
    align-items: center;
    gap: 14px;
    min-height: 56px;
    padding: 0 0 0 16px;
    background: var(--bar);
    color: var(--icon);
  }

  .this-page .lines {
    margin-right: 4px;
  }

  .this-page + .marks,
  .marks + .this-page {
    border-top: 1px solid var(--line);
  }

  /* An open tab's row says what a tap does, as Chrome's does, where ↖ would be. */
  .switch-tab {
    flex: none;
    margin-right: 12px;
    padding: 3px 8px;
    border: 1px solid var(--line);
    border-radius: 10px;
    color: var(--accent);
    font-size: calc(12 * var(--capped-rem) / 16);
    font-weight: 500;
    white-space: nowrap;
  }
</style>
