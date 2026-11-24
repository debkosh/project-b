<script>
  import { tick } from 'svelte';
  import { phone } from '@/phone.js';
  import { drag, keyLabel, requestBarMenu, runMenuItem, send, shell, state as go } from '@/sdk/svelte.svelte.js';

  // The menu bar off the Mac, for the Svelte shells: MenuBar.vue's, the same strip, menus,
  // keys and design.css's floating menu (see it for the whole of how it behaves). hidden is
  // the shell hiding its bars, for a page in full screen or the welcome.
  let { hidden = false } = $props();

  const ours = $derived(go.platform !== 'mac' && !phone);
  const kept = $derived(ours && !!go.prefs.showMenuBar && !hidden);
  let moment = $state(false);
  const shown = $derived(kept || (ours && moment));
  const menus = $derived(go.barMenu.filter((it) => it.items));

  let bar = $state();
  let menuEl = $state();
  let subEl = $state();
  let open = $state(-1);
  let at = $state({ left: 0, top: 0 });
  let sub = $state(-1);
  let subAt = $state({ left: 0, top: 0 });
  let cover = $state(0);

  const check = '<path d="M3.5 8.5 6.5 11.5 12.5 4.5"/>';
  const row = (it) => ({ label: it.title, key: it.action ? keyLabel(go, it.action) : '', disabled: !it.enabled, icon: it.checked ? check : '', run: () => runMenuItem(it) });
  function flat(items) {
    const out = [];
    for (const it of items) {
      if (it.separator) out.push({ separator: true });
      else if (it.items) out.push({ separator: true }, ...flat(it.items), { separator: true });
      else out.push(row(it));
    }
    return out.filter((it, i, all) => !(it.separator && (i === 0 || i === all.length - 1 || all[i - 1].separator)));
  }
  const items = $derived((menus[open]?.items || []).map((it) => (it.separator ? { separator: true } : it.items ? { label: it.title, items: flat(it.items) } : row(it))));
  const subItems = $derived(items[sub]?.items || []);
  const icons = (list) => list.some((i) => i.icon);

  // Kept, the bar is the window's top: the shell's next strip draws no band of its own.
  $effect(() => {
    if (kept) document.documentElement.style.setProperty('--top-band', '0px');
    else document.documentElement.style.removeProperty('--top-band');
    return () => document.documentElement.style.removeProperty('--top-band');
  });

  $effect(() => {
    if (shown) requestBarMenu();
  });

  const titles = () => [...(bar?.querySelectorAll('.menu-bar-title') || [])];
  const entries = (el) => [...(el?.querySelectorAll(':scope > [role="menuitem"]:not([aria-disabled="true"])') || [])];

  function measure() {
    let el = bar?.nextElementSibling;
    while (el && (!el.offsetHeight || el.classList.contains('ui-context-cover'))) el = el.nextElementSibling;
    const top = bar?.parentElement?.getBoundingClientRect().top || 0;
    cover = el ? Math.round(el.getBoundingClientRect().bottom - top) : 0;
  }

  async function openMenu(i, byKeys) {
    if (i < 0 || i >= menus.length) return;
    open = -1;
    sub = -1;
    await tick();
    const r = titles()[i]?.getBoundingClientRect();
    if (!r) return;
    at = { left: r.left, top: r.bottom + 2 };
    open = i;
    await tick();
    const m = menuEl?.getBoundingClientRect();
    if (m && m.right > innerWidth - 4) at = { left: Math.max(4, innerWidth - 4 - m.width), top: at.top };
    if (byKeys) entries(menuEl)[0]?.focus();
    else menuEl?.focus();
  }

  function hide() {
    open = -1;
    sub = -1;
    moment = false;
    if (bar?.contains(document.activeElement)) document.activeElement.blur();
  }

  function closed(e) {
    const i = e ? titles().findIndex((el) => { const r = el.getBoundingClientRect(); return e.clientX >= r.left && e.clientX < r.right && e.clientY >= r.top && e.clientY < r.bottom; }) : -1;
    if (i >= 0 && i !== open) return openMenu(i, false);
    hide();
  }

  function move(by) {
    const n = menus.length;
    if (n) openMenu((open + by + n) % n, true);
  }

  async function openSub(i, focus) {
    if (sub !== i) {
      sub = i;
      await tick();
      const rowEl = entries(menuEl).find((el) => +el.dataset.i === i);
      const r = rowEl?.getBoundingClientRect();
      const m = menuEl.getBoundingClientRect();
      const s = subEl?.getBoundingClientRect();
      if (r && s) subAt = { left: m.right - 2 + s.width > innerWidth - 4 ? Math.max(4, m.left + 2 - s.width) : m.right - 2, top: Math.max(4, Math.min(r.top - 5, innerHeight - 4 - s.height)) };
    }
    if (focus) entries(subEl)[0]?.focus();
  }

  function choose(item, i) {
    if (item.disabled) return;
    if (item.items) return openSub(i, true);
    hide();
    item.run();
  }

  function onMenuKey(e, inSub) {
    const el = inSub ? subEl : menuEl;
    const list = entries(el);
    const i = list.indexOf(document.activeElement);
    const step = (by) => list[i < 0 ? (by > 0 ? 0 : list.length - 1) : (i + by + list.length) % list.length]?.focus();
    const here = i >= 0 ? (inSub ? subItems : items)[+list[i].dataset.i] : null;
    if (e.key === 'ArrowDown') step(1);
    else if (e.key === 'ArrowUp') step(-1);
    else if (e.key === 'ArrowRight' && !inSub && here?.items) openSub(+list[i].dataset.i, true);
    else if (e.key === 'ArrowLeft' && inSub) {
      const was = sub;
      sub = -1;
      entries(menuEl).find((x) => +x.dataset.i === was)?.focus();
    } else if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') move(e.key === 'ArrowRight' ? 1 : -1);
    else if (e.key === 'Enter' || e.key === ' ') {
      if (here) choose(here, +list[i].dataset.i);
    } else if (e.key === 'Escape') {
      if (inSub) {
        sub = -1;
        entries(menuEl)[0]?.focus();
      } else hide();
    } else if (e.key === 'Tab') hide();
    else return;
    e.preventDefault();
    e.stopPropagation();
  }

  function onTitleKey(e, i) {
    const n = menus.length;
    if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') titles()[(i + (e.key === 'ArrowRight' ? 1 : -1) + n) % n]?.focus();
    else if (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown') openMenu(i, true);
    else if (e.key === 'Escape') hide();
    else return;
    e.preventDefault();
  }

  function onFocusOut(e) {
    if (open < 0 && !bar?.contains(e.relatedTarget)) moment = false;
  }

  // Go's ask, Alt alone or F10: the bar comes, the shell takes the keys, and its first title
  // is focused, or its first menu open, once Go's menu is in. Alt again hides a bar it showed.
  let pending = '';
  let seen = go.menuBar.n;
  async function apply() {
    if (!pending || !menus.length) return;
    const what = pending;
    pending = '';
    await tick();
    if (what === 'open') openMenu(0, true);
    else titles()[0]?.focus();
  }
  $effect(() => {
    menus.length;
    apply();
  });
  $effect(() => {
    const n = go.menuBar.n;
    if (n === seen || !ours) return;
    seen = n;
    if (moment && !go.menuBar.open && open < 0) return hide();
    requestBarMenu();
    send({ t: 'focusShell' });
    moment = true;
    pending = go.menuBar.open ? 'open' : 'focus';
    tick().then(measure);
    apply();
  });
</script>

<svelte:window onblur={() => open >= 0 && hide()} />
{#if shown}
  <div
    bind:this={bar}
    class="menu-bar"
    class:moment={!kept}
    style:height={!kept && cover ? `${cover}px` : null}
    use:shell
    role="menubar"
    aria-label="Menu bar"
    tabindex="-1"
    onfocusout={onFocusOut}
  >
    {#each menus as m, i (m.title)}
      <button
        class="menu-bar-title"
        class:open={open === i}
        type="button"
        role="menuitem"
        aria-haspopup="menu"
        aria-expanded={open === i}
        onclick={() => (open === i ? hide() : openMenu(i, false))}
        onkeydown={(e) => onTitleKey(e, i)}
        onpointerenter={() => open >= 0 && open !== i && openMenu(i, false)}>{m.title}</button>
    {/each}
    <div class="menu-bar-rest" use:drag></div>
  </div>
{/if}
{#if open >= 0 && items.length}
  <div class="ui-context-cover" use:shell role="presentation" onpointerdown={(e) => e.target === e.currentTarget && closed(e)} oncontextmenu={(e) => e.preventDefault()}>
    <div bind:this={menuEl} class="ui-menu ui-context-menu" role="menu" tabindex="-1" aria-label={menus[open]?.title} style:left="{at.left}px" style:top="{at.top}px" onkeydown={(e) => onMenuKey(e, false)}>
      {#each items as item, i (i)}
        {#if item.separator}
          <div class="ui-menu-separator" role="separator"></div>
        {:else}
          <div
            role="menuitem"
            tabindex="-1"
            class="ui-menu-item"
            class:open={sub === i}
            data-i={i}
            aria-disabled={item.disabled ? 'true' : undefined}
            aria-haspopup={item.items ? 'menu' : undefined}
            aria-expanded={item.items ? sub === i : undefined}
            onpointermove={(e) => !item.disabled && document.activeElement !== e.currentTarget && e.currentTarget.focus()}
            onpointerenter={() => (item.items ? openSub(i, false) : (sub = -1))}
            onclick={() => choose(item, i)}
          >
            {#if icons(items)}<span class="ui-menu-icon" aria-hidden="true">{#if item.icon}<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">{@html item.icon}</svg>{/if}</span>{/if}
            <span class="ui-menu-label">{item.label}</span>
            {#if item.key}<span class="ui-menu-key">{item.key}</span>{/if}
            {#if item.items}<svg class="ui-menu-chevron" width="6" height="10" viewBox="0 0 6 10" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true"><path d="M1 1l4 4-4 4" /></svg>{/if}
          </div>
        {/if}
      {/each}
    </div>
    {#if sub >= 0}
      <div bind:this={subEl} class="ui-menu ui-context-menu" role="menu" tabindex="-1" aria-label={items[sub]?.label} style:left="{subAt.left}px" style:top="{subAt.top}px" onkeydown={(e) => onMenuKey(e, true)}>
        {#each subItems as item, i (i)}
          {#if item.separator}
            <div class="ui-menu-separator" role="separator"></div>
          {:else}
            <div
              role="menuitem"
              tabindex="-1"
              class="ui-menu-item"
              data-i={i}
              aria-disabled={item.disabled ? 'true' : undefined}
              onpointermove={(e) => !item.disabled && document.activeElement !== e.currentTarget && e.currentTarget.focus()}
              onclick={() => choose(item, i)}
            >
              {#if icons(subItems)}<span class="ui-menu-icon" aria-hidden="true">{#if item.icon}<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">{@html item.icon}</svg>{/if}</span>{/if}
              <span class="ui-menu-label">{item.label}</span>
              {#if item.key}<span class="ui-menu-key">{item.key}</span>{/if}
            </div>
          {/if}
        {/each}
      </div>
    {/if}
  </div>
{/if}

<style>
  /* MenuBar.vue's look, ux's spec. */
  .menu-bar {
    display: flex;
    align-items: center;
    flex: none;
    gap: 2px;
    height: calc(30px + var(--frame-width));
    padding: var(--frame-width) var(--frame-width) 0;
    /* The bar's colour in the middle, the frame's in the band, and the bar's again under the
       line, which is translucent: over the frame's colour it read as a hard teal line. */
    background: linear-gradient(var(--bar), var(--bar)) content-box, linear-gradient(var(--frame), var(--frame)) padding-box, var(--bar);
    border-bottom: 1px solid var(--line);
    box-sizing: border-box;
    /* Above what a shell lays over its page, as Curve's command bar's dimming. */
    position: relative;
    z-index: 2;
    font: 13px/1 var(--sans, system-ui, sans-serif);
  }

  .menu-bar.moment {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    z-index: 50;
    box-shadow: 0 2px 8px rgb(0 0 0 / 14%);
  }

  /* The bar's own colour, not the band's, between the band and the first title. */
  .menu-bar-title:first-child {
    margin-left: 6px;
  }

  .menu-bar-title {
    height: 24px;
    padding: 0 10px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: var(--text);
    font: inherit;
    font-size: 13px;
    white-space: nowrap;
  }

  .menu-bar-title:hover {
    background: var(--accent-tint);
  }

  .menu-bar-title.open {
    background: var(--accent-soft);
  }

  .menu-bar-title:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  .menu-bar-rest {
    flex: 1;
    align-self: stretch;
  }
</style>
