<script>
  import { answerDialog, dialogFor, focusPage, send, shell, state as go } from '@/sdk/svelte.svelte.js';

  // A page's alert, confirm, prompt, leave-page question or sign-in (docs/daily-driver.md),
  // in the middle of the screen over the dimmed window, as a phone's dialogs are. Only the
  // shown tab's: another tab's waits until its tab shows. Go words it; the page waits until
  // it is answered. The phone's back key answers no (App's backKey calls cancel). Its title
  // and its buttons stay put, and what is between them scrolls when the card has little
  // room, as over the keyboard.
  const dialog = $derived(dialogFor(go, go.active));
  let text = $state('');
  let password = $state('');
  let field = $state();
  // Where the host asking sits in a sign-in's words, set in bold; -1 for none.
  const hostAt = $derived(dialog?.host ? dialog.message.indexOf(dialog.host) : -1);

  // Each dialog starts afresh, with the keyboard in its field if it has one.
  $effect(() => {
    const d = dialog;
    if (!d) return;
    text = d.defaultText || '';
    password = '';
    send({ t: 'focusShell' });
    queueMicrotask(() => field?.focus());
  });

  function answer(yes) {
    const d = dialog;
    if (!d) return;
    answerDialog(d.id, yes, yes ? text : '', yes ? password : '');
    focusPage();
  }

  // The back key's way out: as Cancel.
  export function cancel() {
    answer(false);
  }

  function enter(e) {
    if (e.key === 'Enter') {
      e.preventDefault();
      answer(true);
    }
  }
</script>

{#if dialog}
  <div class="scrim" use:shell>
    {#key dialog.id}
      <div class="card" role="alertdialog" aria-modal="true" aria-label={dialog.title}>
        <h2>{dialog.title}</h2>
        <div class="body">
        {#if dialog.message}
          <p class="words">
            {#if hostAt >= 0}{dialog.message.slice(0, hostAt)}<b>{dialog.host}</b>{dialog.message.slice(hostAt + dialog.host.length)}{:else}{dialog.message}{/if}
          </p>
        {/if}
        {#if dialog.kind === 'prompt'}
          <input bind:this={field} bind:value={text} class="text-field" type="text" aria-label={dialog.message || 'Answer'} enterkeyhint="done" onkeydown={enter} />
        {/if}
        {#if dialog.kind === 'signin'}
          {#if dialog.realm}<p class="realm">The site says: “{dialog.realm}”</p>{/if}
          <label for="m-sign-in-user">Username</label>
          <input id="m-sign-in-user" bind:this={field} bind:value={text} class="text-field" type="text" autocomplete="username" autocapitalize="off" spellcheck="false" enterkeyhint="next" />
          <label for="m-sign-in-password">Password</label>
          <input id="m-sign-in-password" bind:value={password} class="text-field" type="password" autocomplete="current-password" enterkeyhint="done" onkeydown={enter} />
          {#if dialog.insecure}<p class="insecure">This connection isn't secure: your password will be sent as plain text.</p>{/if}
        {/if}
        </div>
        <div class="buttons">
          {#if dialog.cancel}<button type="button" class="text-button" onclick={() => answer(false)}>{dialog.cancel}</button>{/if}
          <button type="button" class="text-button primary" onclick={() => answer(true)}>{dialog.ok}</button>
        </div>
      </div>
    {/key}
  </div>
{/if}

<style>
  /* Over the whole window, sheets and bars too: the page waits on it. */
  .scrim {
    position: fixed;
    inset: 0;
    z-index: 30;
    display: grid;
    place-items: center;
    padding: 24px;
    background: color-mix(in srgb, var(--bar) 70%, transparent);
  }

  .card {
    width: 100%;
    max-width: 400px;
    max-height: 100%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    padding: 24px 24px 12px;
    box-sizing: border-box;
    border-radius: 16px;
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow-float);
    animation: grow var(--motion) var(--ease);
  }

  /* Only this scrolls; wider and taller than what it holds, so a field's focus ring isn't cut. */
  .body {
    min-height: 0;
    margin: -4px -24px;
    padding: 4px 24px;
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  h2 {
    flex: none;
    margin: 0 0 12px;
    font-size: 17px;
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .words {
    margin: 0;
    color: var(--muted);
    font-size: 15px;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .realm {
    margin: 4px 0 0;
    color: var(--muted);
    font-size: 13px;
    overflow-wrap: anywhere;
  }

  label {
    display: block;
    margin: 16px 0 4px;
    color: var(--muted);
    font-size: 13px;
  }

  .text-field {
    width: 100%;
    height: 48px;
    box-sizing: border-box;
    font-size: 16px;
  }

  .words + .text-field {
    margin-top: 16px;
  }

  .insecure {
    margin: 12px 0 0;
    color: var(--danger);
    font-size: 13px;
  }

  .buttons {
    flex: none;
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
  }

  .buttons button {
    min-width: 64px;
    height: 48px;
    padding: 0 16px;
    font-size: 15px;
  }

  /* A fade only: the card is its own size from the first frame. */
  @keyframes grow {
    from { opacity: 0; }
  }
</style>
