<script>
  // A site's mark where it has no favicon to show, as a bookmark or a site visited has none:
  // the first letter of its name, else of its host, on a colour its host gives it, the
  // same each time.
  let { url, title = '', size = 56 } = $props();

  function host(u) {
    try {
      return new URL(u).host.replace(/^www\./, '') || u;
    } catch {
      return u;
    }
  }

  const h = $derived(host(url || ''));
  const hue = $derived([...h].reduce((n, c) => (n * 31 + c.charCodeAt(0)) % 360, 0));
</script>

<!-- A small one's letter is larger for its size, so it still reads. -->
<span class="mark" style:--hue={hue} style:--size="{size}px" style:font-size="{Math.round(size * (size < 32 ? 0.6 : 0.36))}px" aria-hidden="true">{(title.trim()[0] || h[0] || '?').toUpperCase()}</span>

<style>
  .mark {
    flex: none;
    display: grid;
    place-items: center;
    width: var(--size);
    height: var(--size);
    border-radius: calc(var(--size) * 0.28);
    background: hsl(var(--hue) 45% 42%);
    color: #fff;
    font-weight: 600;
    line-height: 1;
  }
</style>
