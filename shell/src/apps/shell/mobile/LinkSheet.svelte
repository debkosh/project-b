<script>
  import { chooseContextItem, closeContextMenu, state as go } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';
  import Sheet from './Sheet.svelte';

  // What a long press on a link or an image in the page offers, drawn by the shell since a
  // phone has no menu of its own, in the dense phone sheet (docs/design-system.md, On a
  // phone): a one-line head, the most used actions as a row of tiles, the rest as rows. A link
  // gets New tab, In group, Copy link, Download and Share; an image New tab, Copy image,
  // Download and Share, its picture in the head once Go has it. A linked image has the link's
  // tiles, then the image's under a hairline. Opening is the shell's own, behind, so the group
  // strip gathers the tabs; the rest are Go's actions (docs/daily-driver.md, Right-click),
  // shown only when Go offers them.
  let { menu, onclose, onopen } = $props();
  let list = $state('');
  const link = $derived(menu.link || '');
  const src = $derived(menu.media === 'image' ? menu.src || '' : '');
  const workspaces = $derived(go.workspaces || []);
  const offered = $derived(new Set((menu.actions || []).map((a) => a.id)));
  const bare = (u) => u.replace(/^https?:\/\/(www\.)?/, '');
  const address = $derived(bare(link || src));
  // The head's name: the link's words, or an image's file name, else the address alone.
  const name = $derived(
    (link && menu.linkText?.trim()) || (!link && src && decodeURIComponent(src.split(/[?#]/)[0].split('/').pop() || '')) || '',
  );

  const linkTiles = $derived(
    [
      offered.has('link.newTab') && { icon: 'tab', label: 'New tab', open: { text: link, background: true } },
      offered.has('link.newTab') && { icon: 'group', label: 'In group', open: { text: link, groupWith: menu.tab, background: true } },
      offered.has('link.copy') && { icon: 'copy', label: 'Copy link', id: 'link.copy' },
      offered.has('link.download') && { icon: 'download', label: 'Download', id: 'link.download' },
      offered.has('link.share') && { icon: 'share', label: 'Share', id: 'link.share' },
    ].filter(Boolean),
  );
  const imageTiles = $derived(
    [
      offered.has('image.newTab') && { icon: 'tab', label: 'New tab', open: { text: src, background: true } },
      offered.has('image.copy') && { icon: 'copy', label: 'Copy image', id: 'image.copy' },
      offered.has('image.download') && { icon: 'download', label: 'Download', id: 'image.download' },
      offered.has('image.share') && { icon: 'share', label: 'Share', id: 'image.share' },
    ].filter(Boolean),
  );
  const rows = $derived(
    [
      offered.has('link.newTab') && workspaces.length > 1 && { icon: 'workspace', label: 'Open in workspace', more: true },
      offered.has('link.copyText') && { icon: 'text', label: 'Copy link text', id: 'link.copyText' },
      offered.has('image.copyAddress') && { icon: 'link', label: 'Copy image address', id: 'image.copyAddress' },
    ].filter(Boolean),
  );

  // Go keeps the menu until it hears how it went, and sends it again with the image's
  // picture: closed, the sheet stays closed.
  function close() {
    closeContextMenu(menu);
    onclose();
  }

  function run(item) {
    if (item.more) {
      list = 'workspaces';
    } else if (item.open) {
      closeContextMenu(menu);
      onopen(item.open);
    } else {
      chooseContextItem(menu, item.id);
      onclose();
    }
  }
</script>

<!-- A linked picture's two rows have the same words, so each says whose it is. -->
{#snippet tiles(items, label)}
  <div class="set" role="group" aria-label={label}>
    {#if linkTiles.length && imageTiles.length}<div class="heading" aria-hidden="true">{label}</div>{/if}
    <div class="tiles">
      {#each items as item (item.label)}
        <button class="tile" role="menuitem" onclick={() => run(item)}>
          <Icon name={item.icon} /><span>{item.label}</span>
        </button>
      {/each}
    </div>
  </div>
{/snippet}

<Sheet label={link ? 'Link' : 'Image'} onclose={close}>
  <div class="head">
    {#if src}
      <span class="thumb">
        {#if menu.thumb}<img src={menu.thumb} alt="" />{:else}<Icon name="image" />{/if}
      </span>
    {/if}
    <span class="words">
      {#if name}<span class="name">{name}</span>{/if}
      <span class="url" class:alone={!name}>{address}</span>
    </span>
  </div>
  <div role="menu" aria-label={link ? 'Link' : 'Image'}>
    {#if list === 'workspaces'}
      <div class="rows">
        {#each workspaces as w (w.id)}
          <button class="row" role="menuitem" onclick={() => run({ open: { text: link, workspace: w.id, background: true } })}>
            <span class="dot" style:background={w.color || 'var(--muted)'}></span>
            <span class="label">{w.name}</span>
          </button>
        {/each}
      </div>
    {:else}
      {#if linkTiles.length}{@render tiles(linkTiles, 'Link')}{/if}
      {#if imageTiles.length}{@render tiles(imageTiles, 'Image')}{/if}
      {#if rows.length}
        <div class="rows">
          {#each rows as item (item.label)}
            <button class="row" role="menuitem" onclick={() => run(item)}>
              <Icon name={item.icon} /><span class="label">{item.label}</span>
              {#if item.more}<Icon name="chevron" />{/if}
            </button>
          {/each}
        </div>
      {/if}
    {/if}
  </div>
</Sheet>

<style>
  .head {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 16px 10px;
    border-bottom: 1px solid var(--line);
  }

  .thumb {
    flex: none;
    display: grid;
    place-items: center;
    width: 40px;
    height: 40px;
    overflow: hidden;
    border-radius: 6px;
    background: var(--bar);
    color: var(--muted);
  }

  .thumb img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .words {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .name,
  .url {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .name {
    color: var(--text);
    font-size: 14px;
    font-weight: 600;
  }

  .url {
    color: var(--muted);
    font-size: 12px;
  }

  .url.alone {
    color: var(--text);
    font-size: 14px;
  }

  .tiles {
    display: flex;
    justify-content: space-around;
    padding: 6px 4px;
  }

  .set + .set,
  .rows {
    border-top: 1px solid var(--line);
  }

  .heading {
    display: flex;
    align-items: flex-end;
    height: 24px;
    padding: 0 16px;
    color: var(--muted);
    font-size: 12px;
  }

  [role='menu'] {
    padding-bottom: 8px;
  }

  .tile {
    display: flex;
    flex: 1;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    max-width: 80px;
    height: 64px;
    padding: 0;
    border: 0;
    border-radius: 8px;
    background: none;
    color: var(--icon);
  }

  .tile span {
    color: var(--text);
    font-size: 11.5px;
    white-space: nowrap;
  }

  .rows {
    display: flex;
    flex-direction: column;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 16px;
    height: 48px;
    padding: 0 16px;
    border: 0;
    background: none;
    color: var(--icon);
    font-size: 14px;
    text-align: left;
  }

  .label {
    flex: 1;
    color: var(--text);
  }

  .dot {
    width: 10px;
    height: 10px;
    margin: 0 5px;
    border-radius: 50%;
  }

  button:active {
    background: var(--accent-tint);
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring-inset);
  }
</style>
