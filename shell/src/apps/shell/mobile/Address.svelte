<script>
  import { focusPage, send, shell } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';
  import { popupsWord } from '@/sdk/sites.js';
  import { nextQuery } from './queries.js';

  // The address pill. At rest it shows the host and the page's lock; a tap turns it into
  // the field, with the whole address selected. Typing
  // asks Go for suggestions, listed away from the field towards the page, the closest next
  // to it: a tap on one goes there, its ↖ puts its address in the field. Enter goes; Escape
  // or a press in the page puts the address back.
  let { tab, top, onedit, onsite } = $props();
  let input;
  let editing = $state(false);
  let text = $state('');
  let items = $state([]);
  // The id of the last suggest sent: only its answer is listed.
  let asked = 0;

  const host = $derived.by(() => {
    if (!tab.url) return '';
    try {
      const u = new URL(tab.url);
      return u.protocol === 'http:' || u.protocol === 'https:' ? u.host.replace(/^www\./, '') : tab.url;
    } catch {
      return tab.url;
    }
  });
  // What went wrong with the page, said at the pill's end.
  const status = $derived(tab.blocked ? 'Blocked' : tab.crashed ? 'Crashed' : tab.error ? 'Failed' : '');
  // The lock only for a page that shows: one that failed, crashed or was blocked has the
  // warning, beside the pill's word for what went wrong.
  const secure = $derived(status ? 'warn' : tab.url.startsWith('https:') ? 'lock' : tab.url.startsWith('http:') ? 'warn' : '');
  // The list from the field outwards: under a field on top, over one at the bottom with
  // the first suggestion nearest it.
  const shown = $derived(top ? items : [...items].reverse());

  $effect(() => onedit(editing));

  function begin() {
    if (editing) return;
    editing = true;
    text = tab.url;
    // After the value is in, so the whole of it is selected.
    queueMicrotask(() => input.select());
  }

  function end() {
    editing = false;
    items = [];
    asked = 0;
  }

  function oninput() {
    if (text.trim() === '') {
      items = [];
      asked = 0;
    } else {
      send({ t: 'suggest', id: (asked = nextQuery()), text });
    }
  }

  function go(value) {
    if (value.trim() === '') return;
    send({ t: 'navigate', text: value.trim() });
    leave();
  }

  function leave() {
    input.blur();
    focusPage();
  }

  function fill(url) {
    text = url;
    input.focus();
    oninput();
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
    if (m.id === asked && editing) items = m.items;
  }
</script>

<div class="address" class:editing>
  {#if !editing && secure}
    <button type="button" class="mark" aria-label="Site settings" onclick={onsite}><Icon name={secure} size={16} /></button>
  {/if}
  <input
    bind:this={input}
    value={editing ? text : host}
    oninput={(e) => {
      text = e.currentTarget.value;
      oninput();
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
      if (e.key === 'Enter') go(text);
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
  {#if editing && items.length}
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <ul class="suggestions" class:below={top} use:shell aria-label="Suggestions" onmousedown={(e) => e.preventDefault()}>
      {#each shown as item (item.url)}
        <li>
          <button class="go" onclick={() => go(item.url)}>
            <Icon name={item.title ? 'globe' : 'search'} />
            <span class="lines">
              <span class="title">{item.title || item.url}</span>
              {#if item.title}<span class="url">{item.url.replace(/^https?:\/\/(www\.)?/, '')}</span>{/if}
            </span>
          </button>
          <button class="fill" aria-label="Edit {item.url}" onclick={() => fill(item.url)}><Icon name="fill" /></button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

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
    font-size: 14px;
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
    font-size: 13px;
    font-weight: 600;
  }

  .status {
    flex: none;
    padding: 2px 8px;
    border: 1px solid var(--danger);
    border-radius: 10px;
    color: var(--danger);
    font-size: 12px;
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

  .suggestions {
    position: absolute;
    /* The toolbar's width: the 8px before the pill, and the count and ⋯ after it, 48px each
       with a 4px gap before each. */
    left: -8px;
    right: -104px;
    bottom: calc(100% + 4px);
    z-index: 5;
    margin: 0;
    padding: 0;
    list-style: none;
    background: var(--surface);
    border-top: 1px solid var(--line);
  }

  .suggestions.below {
    top: calc(100% + 4px);
    bottom: auto;
    border-top: 0;
    border-bottom: 1px solid var(--line);
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
    font-size: 14px;
  }

  .url {
    color: var(--muted);
    font-size: 12px;
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
</style>
