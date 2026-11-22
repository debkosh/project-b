<script>
  import { send, shell, state } from '@/sdk/svelte.svelte.js';

  // The saved logins Go offers at a field of the page the person pressed
  // (docs/password-autofill.md), as a phone has them: chips in a bar over the foot of the
  // page, which is just above the keyboard while it is up, as the bars under the page step
  // aside then. It is the shell's, outside the page. A tap on a chip fills that login;
  // nothing is filled before. Manage, at its end, opens Settings at Passwords. For a field
  // in a frame of another site, a line over the chips says whose logins they are.
  let { tab, hidden } = $props();

  const offer = $derived(!hidden && state.passwordOffer && Number(state.passwordOffer.tab) === tab ? state.passwordOffer : null);
  const pageHost = $derived.by(() => {
    try {
      return new URL(state.tabs.find((t) => t.id === tab).url).host;
    } catch {
      return '';
    }
  });
  // A long host keeps its two ends, where the site is.
  const short = (host) => (host.length > 24 ? `${host.slice(0, 9)}…${host.slice(-14)}` : host);

  // A tap in its first half second is no pick: a page can't bring the bar up under a tap
  // already on its way.
  let since = 0;
  let last = 0;
  $effect(() => {
    if (offer && offer.id !== last) {
      last = offer.id;
      since = performance.now();
    }
  });
  function pick(login) {
    if (performance.now() - since < 500) return;
    send({ t: 'passwordPick', offer: offer.id, login: login.id });
  }
  function manage() {
    send({ t: 'passwordDismiss', offer: offer.id });
    send({ t: 'passwordsManage' });
  }

  // A tap on the bar takes the keyboard down, as a touch on the shell does, and the bar
  // goes down with it while the finger is still on the glass: the click that follows is
  // aimed at where the chip was, and is lost. So a tap is the finger coming up where it
  // went down, told to the control it went down on, which keeps the finger wherever the
  // control goes. A finger that moved was scrolling the chips. The click that would follow
  // the finger is stopped, as it would land on whatever the bar left there; a click with
  // no finger before it still serves a screen reader.
  function tap(el, run) {
    let down = null;
    let tapped = 0;
    const start = (e) => (down = { id: e.pointerId, x: e.clientX, y: e.clientY });
    const cancel = () => (down = null);
    const end = (e) => {
      const d = down;
      down = null;
      if (!d || d.id !== e.pointerId || Math.hypot(e.clientX - d.x, e.clientY - d.y) > 10) return;
      tapped = performance.now();
      run();
    };
    // The finger's own end comes after its pointer's: stopping it stops its click.
    const lifted = (e) => {
      if (performance.now() - tapped < 100) e.preventDefault();
    };
    const click = () => {
      if (performance.now() - tapped > 700) run();
    };
    el.addEventListener('pointerdown', start);
    el.addEventListener('pointerup', end);
    el.addEventListener('pointercancel', cancel);
    el.addEventListener('touchend', lifted, { passive: false });
    el.addEventListener('click', click);
    return {
      update: (next) => (run = next),
      destroy() {
        el.removeEventListener('pointerdown', start);
        el.removeEventListener('pointerup', end);
        el.removeEventListener('pointercancel', cancel);
        el.removeEventListener('touchend', lifted);
        el.removeEventListener('click', click);
      },
    };
  }
</script>

{#if offer}
  <div class="slot" use:shell role="group" aria-label={`Saved passwords for ${offer.site}`}>
    {#if offer.framed}
      <div class="whose">This sign-in is from <b>{offer.site}</b>{#if pageHost}, shown inside {pageHost}{/if}.</div>
    {/if}
    {#if offer.insecure}
      <div class="whose">This page isn't secure: what you fill could be read on the way.</div>
    {/if}
    <div class="bar">
      <div class="chips">
        {#each offer.logins as l, i (l.id)}
          <button class="chip" class:at={offer.at === i} aria-current={offer.at === i || undefined} use:tap={() => pick(l)}>
            <svg class="key" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <circle cx="5.25" cy="10.75" r="3.25" />
              <path d="M7.6 8.4 13.75 2.25M11 5l1.75 1.75M13 3l1.5 1.5" />
            </svg>
            <span class="name" class:none={!l.username}>{l.username || 'No username'}</span>
            {#if l.savedFor}<span class="for" title={l.savedFor}>saved for {short(l.savedFor)}</span>{/if}
          </button>
        {/each}
      </div>
      <button class="text-button quiet manage" aria-label="Manage passwords" use:tap={manage}>Manage</button>
    </div>
  </div>
{/if}

<style>
  /* Over the foot of the page, above the bars while they lie over it (--covered,
     App.svelte), and over a download's bar. */
  .slot {
    position: absolute;
    left: 0;
    right: 0;
    bottom: var(--covered, 0px);
    z-index: 4;
    background: var(--bar);
    border-top: 1px solid var(--line);
  }

  .whose {
    padding: 8px 16px 0;
    color: var(--muted);
    font-size: 12px;
    line-height: 16px;
  }

  .whose b {
    color: var(--text);
    font-weight: 600;
  }

  .bar {
    display: flex;
    align-items: center;
    height: 56px;
  }

  /* The chips run off the row's end and scroll sideways under a finger: the row fades into
     the bar there, so a chip that runs off reads as more to come, not as one cut. */
  .chips {
    flex: 1;
    min-width: 0;
    display: flex;
    gap: 8px;
    padding: 0 28px 0 12px;
    overflow-x: auto;
    scrollbar-width: none;
    mask-image: linear-gradient(to right, #000 calc(100% - 28px), transparent);
  }

  /* A chip: 40px drawn, in the bar's 56. */
  .chip {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
    max-width: 280px;
    height: 40px;
    padding: 0 14px 0 12px;
    border: 1px solid var(--line);
    border-radius: 20px;
    background: var(--surface);
    color: var(--text);
    font: inherit;
  }

  .chip.at {
    background: var(--accent-tint);
  }

  .key {
    flex: none;
    color: var(--icon);
  }

  .name,
  .for {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .for {
    flex: none;
    color: var(--muted);
    font-size: 12px;
  }

  .none {
    color: var(--muted);
  }

  /* Apart from the logins, after a hairline: words, so it never reads as one more login. */
  .manage {
    flex: none;
    height: 48px;
    margin: 0 4px 0 0;
    padding: 0 12px;
    border-left: 1px solid var(--line);
    border-radius: 0;
    color: var(--muted);
  }
</style>
