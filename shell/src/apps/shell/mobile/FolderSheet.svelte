<script>
  import BookmarkList from './BookmarkList.svelte';
  import Sheet from './Sheet.svelte';

  // A folder of the bookmarks bar, from the bottom: what it holds, its folders a tap deeper,
  // a tap on a bookmark opening it (onopen, with its address).
  let { list, folder, onclose, onopen } = $props();
  const title = $derived(list.find((e) => e.id === folder)?.title || '');
</script>

<Sheet label={title} {onclose}>
  <div class="folder">
    <BookmarkList {list} root={folder} {onopen} />
  </div>
</Sheet>

<style>
  .folder {
    padding: 0 12px 8px;
  }
</style>
