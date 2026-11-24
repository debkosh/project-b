<script>
  import { tick, untrack } from 'svelte';
  import { closeFind, findNext, findText, focusPage, keyLabel, send, shell, state as go } from '@/sdk/svelte.svelte.js';
  import { pushEscape } from '@/ui/escape.js';

  // Find in page (docs/daily-driver.md): a bar hung from the page's top right while the
  // active tab's search is open, each tab keeping its own. What is typed searches as it
  // changes, with the count at the field's end; Enter shows the next match and Shift+Enter
  // the one before; Escape closes it and gives the page the keyboard back. focus() is the
  // find key's answer.
  let field = $state();
  let text = $state('');

  // The tab whose search shows, or 0.
  const tab = $derived(go.find?.tab === go.active ? go.active : 0);
  const missed = $derived(!!go.find?.text && !go.find.matches);
  const count = $derived(!go.find?.text ? '' : go.find.matches ? `${go.find.active} of ${go.find.matches}` : 'No results');
  // A button's tooltip, with the key the keymap gives its action, when it has one.
  const tip = (title, action) => (keyLabel(go, action) ? `${title} (${keyLabel(go, action)})` : title);

  // The field follows Go's text when a tab's search shows, not while typing.
  $effect(() => {
    if (tab) text = untrack(() => go.find.text);
  });

  $effect(() => {
    if (tab) return pushEscape(close);
  });

  export function focus() {
    send({ t: 'focusShell' });
    tick().then(() => {
      field.focus();
      field.select();
    });
  }

  function close() {
    closeFind(tab);
    focusPage();
  }

  function onkeydown(e) {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    findNext(tab, !e.shiftKey);
  }
</script>

{#if tab}
  <div class="find-bar" use:shell role="search" aria-label="Find in page">
    <div class="box" class:missed>
      <input bind:this={field} bind:value={text} class="text-field" type="text" aria-label="Find in page" placeholder="Find in page" spellcheck="false" oninput={() => findText(tab, text)} {onkeydown} />
      <span class="count" role="status">{count}</span>
    </div>
    <button type="button" class="icon-button" aria-label="Previous match" title={tip('Previous', 'find.previous')} disabled={!go.find.matches} onclick={() => findNext(tab, false)}>
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M4 10l4-4 4 4" /></svg>
    </button>
    <button type="button" class="icon-button" aria-label="Next match" title={tip('Next', 'find.next')} disabled={!go.find.matches} onclick={() => findNext(tab, true)}>
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M4 6l4 4 4-4" /></svg>
    </button>
    <button type="button" class="icon-button" aria-label="Close find bar" title="Close (Esc)" onclick={close}>
      <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.5"><path d="m2 2 6 6M8 2 2 8" /></svg>
    </button>
  </div>
{/if}

<style>
  /* A plain panel hung from the page's top right edge, a hairline round it. */
  .find-bar {
    position: absolute;
    top: 0;
    right: 16px;
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 6px 6px 6px 8px;
    border: 1px solid var(--line);
    border-top: 0;
    background: var(--surface);
  }

  /* The count sits at the field's own right end. */
  .box {
    position: relative;
    margin-right: 4px;
  }

  .text-field {
    width: 260px;
    height: 28px;
    padding-right: 84px;
  }

  .count {
    position: absolute;
    top: 50%;
    right: 10px;
    transform: translateY(-50%);
    color: var(--muted);
    font-size: 12px;
    white-space: nowrap;
    pointer-events: none;
  }

  /* No match: the design system's error state, so a miss shows without reading. */
  .missed .text-field {
    border-color: var(--danger);
  }

  .missed .count {
    color: var(--danger);
  }
</style>
