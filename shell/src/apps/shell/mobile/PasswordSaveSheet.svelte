<script>
  import { send, state as go } from '@/sdk/svelte.svelte.js';
  import Sheet from './Sheet.svelte';

  // The offer to save a password on a phone (docs/password-autofill.md), after a sign-in
  // went through: the shell's sheet, as the bookmark's is, with the username, the password
  // as dots, which Show shows as the person typed it a moment ago, Save or Update, Not now,
  // and Never for this site. A tap outside it, or a swipe down, is Not now.
  let { offer } = $props();
  let showing = $state(false);
  const shown = $derived(go.passwordSaveShown?.offer === offer.id ? go.passwordSaveShown.password : '');

  const answer = (choice) => send({ t: 'passwordSaveAnswer', offer: offer.id, choice });
  function show() {
    showing = !showing;
    if (showing && !shown) send({ t: 'passwordSaveShow', offer: offer.id });
  }
</script>

<Sheet label={offer.update ? 'Update password?' : 'Save password?'} onclose={() => answer('dismiss')}>
  <form
    onsubmit={(e) => {
      e.preventDefault();
      answer('save');
    }}
  >
    <h2>{offer.update ? 'Update password?' : 'Save password?'}</h2>
    <p class="site">For <b>{offer.site}</b></p>
    <div class="row">
      <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <circle cx="8" cy="5.25" r="2.75" />
        <path d="M2.75 14a5.25 5.25 0 0 1 10.5 0" />
      </svg>
      <span class="value" class:none={!offer.username}>{offer.username || 'No username'}</span>
    </div>
    <div class="row">
      <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <path d="M5 7V5a3 3 0 0 1 6 0v2M3.75 7h8.5v6.5h-8.5z" />
      </svg>
      <span class="value" class:secret={showing && shown}>{showing && shown ? shown : '••••••••'}</span>
      <button type="button" class="show" aria-label={showing ? 'Hide the password' : 'Show the password'} onclick={show}>{showing ? 'Hide' : 'Show'}</button>
    </div>
    <div class="buttons">
      <button type="button" class="never" onclick={() => answer('never')}>Never for this site</button>
      <button type="button" onclick={() => answer('dismiss')}>Not now</button>
      <button type="submit" class="primary">{offer.update ? 'Update' : 'Save'}</button>
    </div>
  </form>
</Sheet>

<style>
  /* Laid out as the bookmark's sheet is: 20px sides, 48px rows and buttons. */
  h2 {
    margin: 0;
    padding: 4px 20px 2px;
    font-size: 16px;
    font-weight: 600;
  }

  .site {
    margin: 0;
    padding: 0 20px 8px;
    color: var(--muted);
    font-size: 13px;
  }

  .site b {
    color: var(--text);
    font-weight: 600;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 48px;
    padding: 0 8px 0 20px;
    font-size: 15px;
  }

  .row > svg {
    flex: none;
    color: var(--icon);
  }

  .value {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .none {
    color: var(--muted);
  }

  .secret {
    font:
      14px/20px ui-monospace,
      SFMono-Regular,
      Menlo,
      monospace;
    user-select: text;
  }

  .buttons {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 8px;
    padding: 12px 20px 4px;
  }

  button {
    min-height: 48px;
    min-width: 48px;
    padding: 0 18px;
    border: 0;
    border-radius: 10px;
    background: none;
    color: var(--text);
    font: inherit;
    font-size: 15px;
  }

  .show {
    padding: 0 12px;
    color: var(--accent);
  }

  /* Never for this site keeps to the left, away from the two answers at the thumb. */
  .never {
    margin-right: auto;
    margin-left: -18px;
    color: var(--muted);
  }

  .primary {
    background: var(--accent);
    color: var(--on-accent);
    font-weight: 600;
  }
</style>
