<script>
  import { shell } from '@/sdk/svelte.svelte.js';
  import { pushEscape } from '@/ui/escape.js';

  // A bottom sheet over the window: the backdrop, a tap on which closes it, and the panel,
  // which a swipe down on its handle closes too. label names it for a screen reader. With a
  // mouse, as in a narrow window on a tablet, Esc closes it too, and its rows light under the
  // pointer (docs/large-screens.md). Escape is a layer of escape.js's while it is open, so the
  // back key closes it the same way, and a layer opened inside it goes first.
  let { label, onclose, children } = $props();
  $effect(() => pushEscape(() => onclose()));
  let start = null;

  function down(e) {
    start = e.clientY;
    e.currentTarget.setPointerCapture(e.pointerId);
  }

  function up(e) {
    if (start !== null && e.clientY - start > 40) onclose();
    start = null;
  }
</script>

<div class="backdrop" use:shell onclick={onclose} role="presentation"></div>
<div class="sheet" use:shell role="dialog" aria-label={label}>
  <div class="handle" onpointerdown={down} onpointerup={up} onpointercancel={() => (start = null)} role="presentation"><i></i></div>
  {@render children()}
</div>

<style>
  .backdrop {
    position: absolute;
    inset: 0;
    z-index: 10;
    background: color-mix(in srgb, var(--bar) 70%, transparent);
  }

  .sheet {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 11;
    max-height: 85%;
    overflow-y: auto;
    overscroll-behavior: contain;
    padding-bottom: 8px;
    background: var(--surface);
    border-radius: 16px 16px 0 0;
    box-shadow: var(--shadow-float);
    animation: rise var(--motion) var(--ease);
  }

  .handle {
    display: grid;
    place-items: center;
    height: 20px;
    touch-action: none;
  }

  .handle i {
    width: 32px;
    height: 4px;
    border-radius: 2px;
    background: var(--line);
  }

  /* Any pointer that hovers, not only the first: a phone or a folded foldable with a mouse
     still names its finger first. */
  @media (any-hover: hover) {
    .sheet :global([role='menuitem']:hover:not(:disabled)) {
      background: var(--accent-tint);
    }
  }

  @keyframes rise {
    from { transform: translateY(100%); }
  }
</style>
