<script>
  import { tick } from 'svelte';
  // The design system's dialog styles: .ui-dialog, its title and its buttons.
  import '@/ui/Dialog.vue';
  import { checkPin, codePath, codeSize, firstControl, trapTab } from '@/ui/passkey.js';
  import { answerPasskey, focusPage } from '@/sdk/svelte.svelte.js';

  // One step of a page's passkey request (docs/passkeys.md), as Go words it: the card of a
  // page dialog of kind passkey, as PasskeyCard.vue is for the Vue shells. Its parent
  // makes a new one for each step ({#key}), so each starts with empty fields. The card
  // stays after an answer: Go sends the next step, or takes the dialog away.
  // dialog: the dialog; active: whether its tab has the keyboard; flat: square and
  // shadowless, for a shell that draws so.
  let { dialog, active = true, flat = false } = $props();

  const step = $derived(dialog.passkey);
  const hostAt = $derived(dialog.host ? dialog.message.indexOf(dialog.host) : -1);
  let card = $state();
  let values = $state(['', '']);
  // The words under each PIN field: the key's, of the PIN it refused, then our own.
  let errors = $state([dialog.passkey.pin?.error || '', '']);

  $effect(() => {
    if (active) tick().then(() => firstControl(card)?.focus());
  });

  function act(action, text = '') {
    answerPasskey(dialog.id, action, text);
    if (action === 'cancel') focusPage();
  }

  function next() {
    const wrong = checkPin(step.pin, values);
    errors = ['', ''];
    if (wrong) {
      errors[wrong.field] = wrong.words;
      card.querySelectorAll('input')[wrong.field].focus();
      return;
    }
    act('pin', values[0]);
  }

  function primary() {
    if (step.primary === 'pin') next();
    else act(step.primary);
  }

  function enter(e) {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    next();
  }
</script>

<div bind:this={card} class="ui-dialog card" class:flat class:wide={step.wide} role="alertdialog" aria-modal="true" aria-label={dialog.title} tabindex="-1" onkeydown={(e) => trapTab(card, e)}>
  <h2 class="ui-dialog-title">{dialog.title}</h2>
  {#if dialog.message}
    <p class="says">
      {#if hostAt >= 0}{dialog.message.slice(0, hostAt)}<b>{dialog.host}</b>{dialog.message.slice(hostAt + dialog.host.length)}{:else}{dialog.message}{/if}
    </p>
  {/if}
  {#if step.pin}
    {#each step.pin.labels as label, i (label)}
      <div class="group">
        <label for="pb-passkey-pin-{dialog.id}-{i}">{label}</label>
        <input
          id="pb-passkey-pin-{dialog.id}-{i}"
          bind:value={values[i]}
          class="text-field"
          type="password"
          autocomplete="off"
          aria-invalid={!!errors[i]}
          aria-describedby={errors[i] ? `pb-passkey-pin-error-${dialog.id}-${i}` : null}
          oninput={() => (errors[i] = '')}
          onkeydown={enter}
        />
        {#if errors[i]}<p id="pb-passkey-pin-error-{dialog.id}-{i}" class="text-field-error" role="alert">{errors[i]}</p>{/if}
      </div>
    {/each}
  {/if}
  {#if step.accounts}
    <div class="accounts">
      {#each step.accounts as account, i (i)}
        <button type="button" class="account" title={account.secondary ? `${account.title}\n${account.secondary}` : account.title} onclick={() => act('account', String(i))}>
          <span class="name" class:unnamed={account.unnamed}>{account.title}</span>
          {#if account.secondary}<span class="secondary name">{account.secondary}</span>{/if}
        </button>
      {/each}
    </div>
  {/if}
  {#if step.code}
    <div class="code">
      <svg role="img" aria-label="QR code for your phone" viewBox="0 0 {codeSize(step.code)} {codeSize(step.code)}" shape-rendering="crispEdges">
        <path d={codePath(step.code)} fill="#000" />
      </svg>
    </div>
    {#if step.bluetooth}
      <div class="bluetooth">
        <span>{step.bluetooth.words}</span>
        {#if step.bluetooth.button}<button type="button" class="text-button" onclick={() => act(step.bluetooth.button.action)}>{step.bluetooth.button.label}</button>{/if}
      </div>
    {/if}
  {/if}
  <div class="ui-dialog-buttons buttons">
    {#if step.other}<button type="button" class="text-button quiet other" onclick={() => act(step.other.action)}>{step.other.label}</button>{/if}
    {#if dialog.cancel}<button type="button" class="text-button" onclick={() => act('cancel')}>{dialog.cancel}</button>{/if}
    {#if dialog.ok}<button type="button" class="text-button primary" onclick={primary}>{dialog.ok}</button>{/if}
  </div>
</div>

<style>
  .card {
    display: block;
    outline: none;
    line-height: 1.4;
  }

  .flat {
    border-radius: 0;
    box-shadow: none;
  }

  /* A card of ours with the way to a security key on its left: wide enough for that,
     Cancel and the primary button on one row. */
  .card.wide {
    width: 440px;
  }

  .says {
    margin: 0;
    overflow-wrap: anywhere;
  }

  /* The card's 16px rhythm: the buttons sit 16px under what is above them, the first field
     16px under the words and 12px between two, each label 4px over its field. */
  .card .ui-dialog-buttons {
    margin-top: 16px;
  }

  .group {
    margin-top: 16px;
  }

  .group + .group {
    margin-top: 12px;
  }

  .group label {
    display: block;
    margin: 0 0 4px;
  }

  .group .text-field {
    width: 100%;
    box-sizing: border-box;
  }

  .card .text-field-error {
    margin: 4px 0 0;
    color: var(--danger);
  }

  /* A key's accounts, in a card of their own 16px under the words: a hairline round it and
     between its rows, each row a button of the card's full width. About five show before
     the list scrolls inside the card, so Cancel stays in view. */
  .accounts {
    max-height: 244px;
    margin-top: 16px;
    border: 1px solid var(--line);
    border-radius: 6px;
    overflow-y: auto;
  }

  .account {
    display: flex;
    flex-direction: column;
    justify-content: center;
    width: 100%;
    height: 48px;
    padding: 8px 12px;
    box-sizing: border-box;
    border: 0;
    background: none;
    color: var(--text);
    font: inherit;
    text-align: left;
    cursor: default;
  }

  .account + .account {
    border-top: 1px solid var(--line);
  }

  /* An account the key has no name for. */
  .unnamed {
    color: var(--muted);
  }

  .flat .accounts {
    border-radius: 0;
  }

  .account:hover {
    background: var(--accent-tint);
  }

  .account:focus-visible {
    outline: none;
    box-shadow: var(--focus-ring-inset);
  }

  .name {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  /* The phone's code: dark on a white tile whatever the theme, never dimmed, its quiet
     zone inside the tile. */
  .code {
    width: 200px;
    height: 200px;
    margin: 16px auto 0;
    border-radius: 4px;
    background: #fff;
    overflow: hidden;
  }

  .code svg {
    display: block;
    width: 100%;
    height: 100%;
  }

  .bluetooth {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 12px;
  }

  .bluetooth span {
    flex: 1;
    color: var(--muted);
    font-size: 12px;
  }

  .bluetooth .text-button {
    flex: none;
    font-size: 13px;
  }

  /* The way to another kind of key sits apart, on the left. */
  .other {
    margin-right: auto;
  }
</style>
