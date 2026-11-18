<script>
  // A site's mark where it has no favicon to show, as a bookmark or a site visited has none:
  // the first letter of its name, else of its host, on its site's tile (tint.js), made for
  // the background it sits on, on: 'surface' or 'bar'.
  import { siteHost, siteTile, tints } from '@/ui/tint.js';

  let { url, title = '', size = 56, on = 'surface' } = $props();

  const h = $derived(siteHost(url || ''));
  const tile = $derived(($tints, siteTile(url || '', on, size)));
</script>

<!-- A small one's letter is larger for its size, so it still reads. -->
<span
  class="mark"
  style:background={tile.background}
  style:box-shadow="inset 0 0 0 1px {tile.border}"
  style:color={tile.letter}
  style:--size="{size}px"
  style:font-size="{Math.round(size * (size < 32 ? 0.6 : 0.36))}px"
  aria-hidden="true">{(title.trim()[0] || h[0] || '?').toUpperCase()}</span
>

<style>
  .mark {
    flex: none;
    display: grid;
    place-items: center;
    width: var(--size);
    height: var(--size);
    border-radius: calc(var(--size) * 0.28);
    font-weight: 600;
    line-height: 1;
  }
</style>
