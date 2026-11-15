<script>
  import { chooseContextItem, closeContextMenu, contextMenuPoint, focusPage, keyLabel, send, shell, state as go } from '@/sdk/svelte.svelte.js';
  import { pushEscape } from '@/ui/escape.js';
  // The design system's floating menu: .ui-menu and its items, and the right-click menu's
  // cover over the window.
  import '@/ui/RowMenu.vue';
  import '@/ui/ContextMenu.vue';

  // The right-click menu over a page (docs/daily-driver.md): Go's actions for what was
  // pressed on, in Go's order, a line between sections, a link's workspaces one item each.
  // Up and Down move, Enter chooses, and Escape, a press outside it or the shell losing the
  // keyboard closes it, which Go hears unless an item was chosen.

  // Where the menu draws a line: between Go's groups, and inside one between these sections.
  const sections = { 'link.copy': 'keep', 'link.saveAs': 'keep', 'edit.undo': 'undo', 'edit.redo': 'undo' };
  // The key each action shows: the keymap's action it shares, or an edit's own.
  const keys = { 'page.back': 'nav.back', 'page.forward': 'nav.forward', 'page.reload': 'nav.reload', 'selection.copy': 'edit.copy' };

  const menu = $derived(go.contextMenu);
  // The menu last chosen from or closed, gone while Go takes it away.
  let done = $state(0);
  const open = $derived(!!menu && menu.id !== done);

  const items = $derived.by(() => {
    const out = [];
    let section = '';
    for (const a of menu?.actions || []) {
      const s = sections[a.id] || a.group;
      if (section && s !== section) out.push({ separator: true });
      section = s;
      if (a.actions) out.push(...a.actions.map((w) => ({ ...w, title: `Open Link in ${w.title}` })));
      else out.push({ ...a, key: keyLabel(go, a.id.startsWith('edit.') ? a.id : keys[a.id]) });
    }
    return out;
  });

  const label = $derived(!menu ? '' : menu.link ? 'Link' : menu.src ? 'Image' : menu.editable ? 'Text field' : menu.selection ? 'Selection' : 'Page');

  $effect(() => {
    if (open) return pushEscape(close);
  });

  function finish(tell) {
    done = menu.id;
    tell(menu);
    focusPage();
  }

  const choose = (a) => finish((m) => chooseContextItem(m, a.id));
  const close = () => finish(closeContextMenu);

  // Shown, the menu goes to the press, flipped left or up to stay 4px inside the window,
  // and takes the keyboard, with no item chosen until a key or the pointer moves.
  function place(el) {
    const p = contextMenuPoint(menu);
    const r = el.getBoundingClientRect();
    el.style.left = `${p.x + r.width > innerWidth - 4 ? Math.max(4, p.x - r.width) : p.x}px`;
    el.style.top = `${p.y + r.height > innerHeight - 4 ? Math.max(4, p.y - r.height) : p.y}px`;
    send({ t: 'focusShell' });
    el.focus();
  }

  function onkeydown(e) {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
    e.preventDefault();
    const all = [...e.currentTarget.querySelectorAll('[role=menuitem]:not(:disabled)')];
    const i = all.indexOf(document.activeElement);
    const by = e.key === 'ArrowDown' ? 1 : -1;
    all[i < 0 ? (by > 0 ? 0 : all.length - 1) : (i + by + all.length) % all.length]?.focus();
  }
</script>

<svelte:window onblur={() => open && close()} />

{#if open && items.length}
  {#key menu.id}
    <div class="ui-context-cover" use:shell role="presentation" onpointerdown={(e) => e.target === e.currentTarget && close()} oncontextmenu={(e) => e.preventDefault()}>
      <div use:place class="ui-menu ui-context-menu menu" role="menu" tabindex="-1" aria-label={label} {onkeydown}>
        {#each items as a}
          {#if a.separator}
            <div class="ui-menu-separator" role="separator"></div>
          {:else}
            <button type="button" class="ui-menu-item" role="menuitem" tabindex="-1" disabled={!a.enabled} onpointermove={(e) => e.currentTarget.focus()} onclick={() => choose(a)}>
              <span class="ui-menu-label">{a.title}</span>
              {#if a.key}<span class="ui-menu-key">{a.key}</span>{/if}
            </button>
          {/if}
        {/each}
      </div>
    </div>
  {/key}
{/if}

<style>
  /* The design system's menu, square and flat, as everything here is. */
  .menu,
  .menu .ui-menu-item {
    border-radius: 0;
  }

  .menu {
    box-shadow: none;
  }
</style>
