<script>
  import { untrack } from 'svelte';
  import { downloadAction, send, shell } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';

  // A download seen as it happens (docs/shells.md, "A download you can see"): a small bar
  // over the foot of the page, above the toolbar. What it says is Go's download news
  // (core/app/downloads.go), the same for every phone shell; this only draws it, and keeps
  // a phone's rules for when: a start only while the app is showing, an end while the app
  // or the bar is, and nothing while hidden, while a sheet is open, which it must not
  // cover, or the Downloads page shows, which says it all already. Hidden, it goes, and
  // says nothing until that ends, so it never comes back over what the person went on to.
  // Away, while the soft keyboard is up, it keeps out of sight but keeps its news, and shows
  // it once the keyboard goes: a download started from the address or a form says so then,
  // its time, and the offer's one showing, starting only as it shows.
  // The first start may offer to show downloads in the system's notifications, with Allow
  // only; shown once, it never comes again. News goes after Go's time for it, 2 seconds, a
  // question 8; a press keeps it while held. A tap anywhere else, or a swipe down or
  // sideways on it, puts it away. The find bar is in the toolbar's place, under it.
  let { hidden, away } = $props();

  // The news shown: Go's message, or null.
  let said = $state(null);
  let bar = $state();
  let timer = 0;
  let start = null;

  function wait() {
    clearTimeout(timer);
    timer = setTimeout(() => (said = null), said.lasts);
  }

  export function news(m) {
    const shown = said?.id === m.id;
    if (hidden) return;
    const showing = document.visibilityState === 'visible';
    if (m.state === 'gone' || (m.state !== 'progress' && !shown && !showing) || (m.state === 'progress' && !showing)) {
      if (shown) dismiss();
      return;
    }
    said = m;
    if (away) return;
    // Held, it waits for the finger to lift.
    if (start === null) wait();
    else clearTimeout(timer);
    if (m.offer) send({ t: 'notificationsOffered' });
  }

  function dismiss() {
    clearTimeout(timer);
    said = null;
  }

  $effect(() => {
    if (hidden) dismiss();
  });

  // The keyboard coming stops the news' time; going, it shows the news and starts it.
  $effect(() => {
    const out = away;
    untrack(() => {
      if (!said) return;
      if (out) clearTimeout(timer);
      else {
        wait();
        if (said.offer) send({ t: 'notificationsOffered' });
      }
    });
  });

  function act(action) {
    if (action === 'allowNotifications') send({ t: 'allowNotifications' });
    else downloadAction(said.id, action);
    dismiss();
  }

  function down(e) {
    start = { x: e.clientX, y: e.clientY };
    clearTimeout(timer);
    e.currentTarget.setPointerCapture(e.pointerId);
  }

  function up(e) {
    if (start === null) return;
    const swiped = e.clientY - start.y > 24 || Math.abs(e.clientX - start.x) > 24;
    start = null;
    if (swiped) dismiss();
    else if (said) wait();
  }
</script>

<svelte:window onpointerdowncapture={(e) => said && !bar?.contains(e.target) && dismiss()} />

<div class="slot" class:away aria-live="polite">
  {#if said}
    <div class="bar" bind:this={bar} use:shell role="group" aria-label="Download" onpointerdown={down} onpointerup={up} onpointercancel={() => ((start = null), said && wait())}>
      <span class="mark" class:failed={said.mark === 'warn'}><Icon name={said.mark} /></span>
      <span class="text">
        <span class="words" class:asking={!!said.ask}>{said.words}</span>
        {#if said.ask}<span class="ask">{said.ask}</span>{/if}
      </span>
      {#each said.actions as a (a.action)}
        <button onclick={() => act(a.action)}>{a.label}</button>
      {/each}
    </div>
  {/if}
</div>

<style>
  /* Above the bars while they lie over the page (--covered, App.svelte). */
  .slot {
    position: absolute;
    left: 8px;
    right: 8px;
    bottom: calc(8px + var(--covered, 0px));
    z-index: 3;
  }

  .slot.away {
    display: none;
  }

  /* Floating: the surface, a hairline, radius 8 and the floating shadow. */
  .bar {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 48px;
    padding: 0 4px 0 16px;
    box-sizing: border-box;
    border: 1px solid var(--line);
    border-radius: 8px;
    background: var(--surface);
    box-shadow: var(--shadow-float);
    color: var(--text);
    touch-action: none;
    animation: rise var(--motion) var(--ease);
  }

  .mark {
    flex: none;
    display: grid;
    color: var(--icon);
  }

  .mark.failed {
    color: var(--danger);
  }

  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    padding: 6px 0;
    font-size: calc(14rem / 16);
    line-height: calc(18rem / 16);
  }

  /* Two lines at most, so a failure's reason isn't cut off; a long name still ends in an
     ellipsis. Over a question, one line, so the question below is always whole. */
  .words {
    display: -webkit-box;
    overflow: hidden;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
  }

  .words.asking {
    -webkit-line-clamp: 1;
    word-break: break-all;
  }

  .ask {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  button {
    flex: none;
    height: 48px;
    padding: 0 16px;
    border: 0;
    border-radius: 8px;
    background: none;
    color: var(--icon);
    font: inherit;
    font-weight: 600;
  }

  button:active {
    background: var(--accent-tint);
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring-inset);
  }

  @keyframes rise {
    from { transform: translateY(16px); opacity: 0; }
  }
</style>
