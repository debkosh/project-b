<script>
  import { closeFind, findNext, findText, focusPage, send, shell, state as go } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';

  // Find in page (docs/daily-driver.md), in the toolbar's place while the shown tab's
  // search is open: what is typed searches as it changes, the count at the field's end;
  // the arrows and the keyboard's Enter go to the match after or before; × closes it,
  // as the phone's back key does (App's backKey), and the page has the keyboard again.
  let field = $state();
  let text = $state('');
  const tab = $derived(go.find?.tab);
  const count = $derived.by(() => {
    const f = go.find;
    if (!f || !f.text) return '';
    return f.matches ? `${f.active}/${f.matches}` : '0/0';
  });

  // The field follows Go's text when another tab's search shows, not while typing.
  let shown = 0;
  $effect(() => {
    if (go.find && go.find.tab !== shown) {
      shown = go.find.tab;
      text = go.find.text;
    }
  });

  // focusFind's answer: the keyboard in the field.
  export function focus() {
    send({ t: 'focusShell' });
    queueMicrotask(() => {
      field?.focus();
      field?.select();
    });
  }

  export function close() {
    closeFind(tab);
    focusPage();
  }
</script>

<div class="find" role="search" aria-label="Find in page" use:shell>
  <div class="box" class:missed={!!go.find?.text && !go.find.matches}>
    <input
      bind:this={field}
      bind:value={text}
      type="text"
      aria-label="Find in page"
      placeholder="Find in page"
      autocapitalize="off"
      spellcheck="false"
      enterkeyhint="search"
      oninput={() => findText(tab, text)}
      onkeydown={(e) => {
        if (e.key === 'Enter') {
          e.preventDefault();
          findNext(tab, !e.shiftKey);
        } else if (e.key === 'Escape') close();
      }}
    />
    <span class="count" role="status">{count}</span>
  </div>
  <button aria-label="Previous match" disabled={!go.find?.matches} onmousedown={(e) => e.preventDefault()} onclick={() => findNext(tab, false)}><Icon name="up" /></button>
  <button aria-label="Next match" disabled={!go.find?.matches} onmousedown={(e) => e.preventDefault()} onclick={() => findNext(tab, true)}><Icon name="down" /></button>
  <button aria-label="Close find" onclick={close}><Icon name="close" /></button>
</div>

<style>
  .find {
    position: absolute;
    inset: 0;
    z-index: 4;
    display: flex;
    align-items: center;
    padding: 0 0 0 8px;
    background: var(--bar);
    box-shadow: inset 0 1px var(--line);
  }

  .box {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    height: 40px;
    padding: 0 12px 0 14px;
    border-radius: 20px;
    background: var(--surface);
    box-shadow: inset 0 0 0 1.5px var(--accent);
  }

  .box.missed {
    box-shadow: inset 0 0 0 1.5px var(--danger);
  }

  input {
    flex: 1;
    min-width: 0;
    height: 48px;
    margin: -4px 0;
    padding: 0;
    border: 0;
    outline: none;
    background: none;
    color: var(--text);
    font-family: inherit;
    font-size: 14px;
  }

  .count {
    flex: none;
    color: var(--muted);
    font-size: 13px;
    font-variant-numeric: tabular-nums;
  }

  button {
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

  button:disabled {
    opacity: 0.4;
  }
</style>
