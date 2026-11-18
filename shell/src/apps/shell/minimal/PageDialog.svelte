<script>
  import { tick, untrack } from 'svelte';
  import { answerDialog, dialogFor, focusPage, send, shell, state as go } from '@/sdk/svelte.svelte.js';
  import { pushEscape } from '@/ui/escape.js';
  // The design system's dialog styles: .ui-dialog, its title and its buttons.
  import '@/ui/Dialog.vue';
  import PasskeyCard from '@/ui/PasskeyCard.svelte';

  // The active tab's page asks something (docs/daily-driver.md): the design system's dialog
  // over that page only, so the tabs and the toolbar still work, with Go's title, words and
  // buttons, a field for a prompt, and a sign-in's Username and Password. The page waits
  // until it is answered; another tab shown hides it, and showing the tab again brings it
  // back, as Go keeps it.
  const dialog = $derived(dialogFor(go, go.active));
  const id = $derived(dialog?.id);
  // Where the host asking sits in a sign-in's words, set in bold so it reads first; -1 for none.
  const hostAt = $derived(dialog?.host ? dialog.message.indexOf(dialog.host) : -1);
  let text = $state('');
  let password = $state('');
  let card = $state();
  let field = $state();
  let ok = $state();

  // Each dialog starts afresh, takes the keyboard, and answers no to Escape.
  $effect(() => {
    if (!id) return;
    text = untrack(() => dialog.defaultText) || '';
    password = '';
    send({ t: 'focusShell' });
    tick().then(() => {
      // A passkey's card takes the keyboard itself, at each step.
      if (untrack(() => dialog.kind) === 'passkey') return;
      if (field) {
        field.focus();
        field.select();
      } else {
        ok.focus();
      }
    });
    return pushEscape(() => answer(false));
  });

  function answer(yes) {
    answerDialog(dialog.id, yes, yes ? text : '', yes ? password : '');
    focusPage();
  }

  // Tab and Shift+Tab go round the card's own controls while the page waits.
  function trap(e) {
    if (e.key !== 'Tab') return;
    const all = [...card.querySelectorAll('input, button')];
    const i = all.indexOf(document.activeElement);
    e.preventDefault();
    all[(i + (e.shiftKey ? -1 : 1) + all.length) % all.length].focus();
  }

  function enter(e) {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    answer(true);
  }
</script>

{#if dialog}
  <div class="page-dialog" use:shell>
    {#if dialog.kind === 'passkey'}
      <!-- A passkey request: a new card for each step, square and flat, as everything here is. -->
      {#key dialog.passkey.seq}<PasskeyCard {dialog} flat />{/key}
    {:else}
    {#key dialog.id}
      <div bind:this={card} class="ui-dialog card" role="alertdialog" aria-modal="true" aria-label={dialog.title} tabindex="-1" onkeydown={trap}>
        <h2 class="ui-dialog-title">{dialog.title}</h2>
        {#if dialog.message}
          <p class="message">
            {#if hostAt >= 0}{dialog.message.slice(0, hostAt)}<b>{dialog.host}</b>{dialog.message.slice(hostAt + dialog.host.length)}{:else}{dialog.message}{/if}
          </p>
        {/if}
        {#if dialog.kind === 'prompt'}
          <input bind:this={field} bind:value={text} class="text-field field" type="text" aria-label={dialog.message || 'Answer'} onkeydown={enter} />
        {/if}
        {#if dialog.kind === 'signin'}
          <!-- The realm is the site's own words, so it reads as theirs, not ours. -->
          {#if dialog.realm}<p class="realm">The site says: “{dialog.realm}”</p>{/if}
          <div class="group">
            <label for="pb-sign-in-user">Username</label>
            <input id="pb-sign-in-user" bind:this={field} bind:value={text} class="text-field" type="text" autocomplete="username" spellcheck="false" onkeydown={enter} />
          </div>
          <div class="group">
            <label for="pb-sign-in-password">Password</label>
            <input id="pb-sign-in-password" bind:value={password} class="text-field" type="password" autocomplete="current-password" onkeydown={enter} />
          </div>
          {#if dialog.insecure}<p class="insecure">This connection isn't secure: your password will be sent as plain text.</p>{/if}
        {/if}
        <div class="ui-dialog-buttons">
          {#if dialog.cancel}<button type="button" class="text-button" onclick={() => answer(false)}>{dialog.cancel}</button>{/if}
          <button bind:this={ok} type="button" class="text-button primary" onclick={() => answer(true)}>{dialog.ok}</button>
        </div>
      </div>
    {/key}
    {/if}
  </div>
{/if}

<style>
  /* Over the page, which is the shell's while it waits, so no press reaches it; the page
     fades toward the bar's colour, as behind the extensions' prompt. Under the toolbar's
     panels, as all the page's box holds is. */
  .page-dialog {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: start center;
    padding-top: 64px;
    background: color-mix(in srgb, var(--bar) 70%, transparent);
  }

  /* The design system's dialog, square and flat, as everything here is. */
  .card {
    border-radius: 0;
    box-shadow: none;
    outline: none;
    line-height: 1.4;
  }

  .message {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    max-height: 50vh;
    overflow: auto;
  }

  .field {
    width: 100%;
  }

  /* The card's 16px rhythm: the buttons sit 16px under what is above them. */
  .card .ui-dialog-buttons {
    margin-top: 16px;
  }

  .message + .field {
    margin-top: 16px;
  }

  /* A sign-in: the site's words 4px under ours, 16px to the first field, 12px between the
     two, each label 4px over its field. */
  .realm {
    margin-top: 4px;
    font-size: 12px;
    overflow-wrap: anywhere;
  }

  .group {
    margin-top: 12px;
  }

  .realm + .group,
  .message + .group {
    margin-top: 16px;
  }

  .group .text-field {
    width: 100%;
  }

  .insecure {
    margin-top: 16px;
    color: var(--danger);
    font-size: 12px;
  }
</style>
