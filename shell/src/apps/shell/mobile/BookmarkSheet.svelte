<script>
  import { folders, removeBookmark, saveBookmark } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';
  import Sheet from './Sheet.svelte';

  // The star's panel as a sheet (docs/bookmark-folders.md): what the menu's Bookmark shows
  // once Go has bookmarked the page into the folder last saved in. Its name and folder can
  // change: Done, or Enter in the name, saves them, and the next bookmark goes into that
  // folder too. The back key, a tap outside or a swipe down closes it with the bookmark kept
  // as it was; Remove takes it away.
  let { bookmark, list, added, onclose } = $props();

  let title = $state(bookmark.title);
  let parent = $state(bookmark.parent);
  const places = $derived(folders(list));

  function save(e) {
    e?.preventDefault();
    saveBookmark(bookmark.id, title.trim(), parent);
    onclose();
  }

  function remove() {
    removeBookmark(bookmark.id);
    onclose();
  }
</script>

<Sheet label={added ? 'Bookmarked' : 'Edit bookmark'} {onclose}>
  <form onsubmit={save}>
    <h2>{added ? 'Bookmarked' : 'Edit bookmark'}</h2>
    <label class="field">
      <span>Name</span>
      <input bind:value={title} aria-label="Name" enterkeyhint="done" />
    </label>
    {#if places.length > 1}
      <p class="label" id="bookmark-folder">Folder</p>
      <div class="folders" role="radiogroup" aria-labelledby="bookmark-folder">
        {#each places as f (f.id)}
          <label class="folder" class:on={parent === f.id} style:padding-left="{20 + f.depth * 20}px">
            <input type="radio" name="bookmark-folder" value={f.id} bind:group={parent} />
            <Icon name="folder" />
            <span>{f.title}</span>
          </label>
        {/each}
      </div>
    {/if}
    <div class="buttons">
      <button type="button" class="quiet" onclick={remove}>Remove</button>
      <button type="submit" class="primary">Done</button>
    </div>
  </form>
</Sheet>

<style>
  h2 {
    margin: 0;
    padding: 4px 20px 12px;
    font-size: 16px;
    font-weight: 600;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 0 20px 12px;
  }
  .field span,
  .label {
    font-size: 13px;
    color: var(--muted);
  }
  .field input {
    height: 48px;
    box-sizing: border-box;
    padding: 0 12px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--surface);
    color: var(--text);
    font: inherit;
    font-size: 15px;
  }
  .label {
    margin: 0;
    padding: 4px 20px 6px;
  }
  .folders {
    max-height: 40vh;
    overflow-y: auto;
    border-top: 1px solid var(--line);
    border-bottom: 1px solid var(--line);
  }
  .folder {
    position: relative;
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 48px;
    padding-right: 20px;
    font-size: 15px;
    color: var(--text);
  }
  .folder :global(svg) {
    flex: none;
    color: var(--icon);
  }
  .folder span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .folder.on {
    background: var(--accent-soft);
    font-weight: 600;
  }
  .folder input {
    position: absolute;
    opacity: 0;
    width: 1px;
    height: 1px;
    margin: 0;
  }
  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 12px 20px 4px;
  }
  .buttons button {
    min-height: 48px;
    min-width: 48px;
    padding: 0 18px;
    border: 0;
    border-radius: 10px;
    font: inherit;
    font-size: 15px;
  }
  .quiet {
    background: none;
    color: var(--muted);
  }
  .primary {
    background: var(--accent);
    color: var(--on-accent);
    font-weight: 600;
  }
</style>
