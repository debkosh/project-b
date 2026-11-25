<script>
  import { shell } from '@/sdk/svelte.svelte.js';
  import { inFolder } from '@/ui/bookmarks.js';
  import Icon from './Icon.svelte';
  import Mark from './Mark.svelte';
  import { folderMark, tints } from '@/ui/tint.js';

  // The bookmarks bar, while bookmarksBar is on (docs/bookmark-folders.md, Where bookmarks
  // show): what bookmarksBarFolder holds, the top when that folder is gone, as chips in one
  // row that scrolls sideways, its folders first. A tap on a bookmark opens it (onopen, with
  // its address); on a folder, shows what it holds (onfolder, with its id). It is drawn
  // only while the folder holds something. The shell lays it with its bars, so it hides and
  // comes back with them; top says it sits under the address bar, not over the foot.
  let { list, folder, top = false, onopen, onfolder } = $props();
  const shown = $derived(list.some((e) => e.id === folder && e.kind === 'folder') ? folder : '');
  const entries = $derived(inFolder(list, shown));
</script>

{#if entries.length}
  <div class="marks-bar" class:top use:shell role="toolbar" aria-label="Bookmarks bar">
    {#each entries as e (e.id)}
      {#if e.kind === 'folder'}
        <button class="chip" aria-haspopup="dialog" onclick={() => onfolder(e.id)}><span class="tint" style:color={($tints, folderMark(e.id, 'bar'))}><Icon name="folder" size={18} /></span><span class="name">{e.title}</span></button>
      {:else}
        <button class="chip" onclick={() => onopen(e.url)}><Mark url={e.url} title={e.title} size={20} on="bar" /><span class="name">{e.title || e.url}</span></button>
      {/if}
    {/each}
  </div>
{/if}

<style>
  /* A 48px row, every chip a 48px target, and the hairline on the side the page is. */
  .marks-bar {
    flex: none;
    display: flex;
    align-items: center;
    height: 49px;
    padding: 0 4px;
    box-sizing: border-box;
    overflow-x: auto;
    scrollbar-width: none;
    background: var(--bar);
    border-top: 1px solid var(--line);
  }

  .marks-bar.top {
    border-top: 0;
    border-bottom: 1px solid var(--line);
  }

  .chip {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    max-width: 160px;
    min-width: 48px;
    height: 48px;
    padding: 0 10px;
    border: 0;
    border-radius: 14px;
    background: none;
    color: var(--icon);
    font-size: calc(13 * var(--capped-rem) / 16);
  }

  .name {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: var(--text);
  }

  .chip:active {
    background: var(--press);
  }

  .chip:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring);
  }

  /* A folder's icon in its folder's colour, laid out as the icon alone. */
  .tint {
    display: contents;
  }
</style>
