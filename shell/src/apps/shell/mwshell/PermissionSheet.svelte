<script>
  import { answerPermission, osLine, osTitle, permissionAsk, permissionHost } from '@/sdk/sites.js';
  import Sheet from './Sheet.svelte';

  // A site asking to use its camera, microphone or notifications, on a phone
  // (docs/daily-driver.md): the shell's sheet, as the bookmark's is, the glyph and question at
  // the top, then the answers as full-width rows, the primary last, nearest the thumb: Block,
  // Always allow and Allow this time, or Block and Allow. With prompt.os the OS refused what
  // the site is allowed (docs/media-permissions.md): its words, then Not now and Open
  // settings. The back key, a swipe down or a tap outside is Dismiss, not Block, so the site
  // may ask again; the page stays usable behind it.
  let { prompt } = $props();
  const question = $derived(prompt.os ? osTitle(prompt) : `${permissionHost(prompt)} wants to ${permissionAsk(prompt)}`);
  const glyph = $derived(prompt.kinds.includes('camera') ? 'camera' : prompt.kinds.includes('microphone') ? 'microphone' : 'bell');
  const answer = (choice) => answerPermission(prompt, choice);
</script>

<!-- Escape, and the back key through it, is the sheet's close: Dismiss. -->
<Sheet label={question} onclose={() => answer('dismiss')}>
  <div class="head">
    <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      {#if glyph === 'camera'}
        <rect x="1.75" y="4" width="9" height="8" rx="1.5" />
        <path d="M10.75 7 14.25 5v6l-3.5-2" />
      {:else if glyph === 'microphone'}
        <rect x="5.75" y="1.75" width="4.5" height="8" rx="2.25" />
        <path d="M3.5 7.5a4.5 4.5 0 0 0 9 0M8 12v2.25" />
      {:else}
        <path d="M4 11.5V7a4 4 0 0 1 8 0v4.5l1 1H3zM6.5 14h3" />
      {/if}
    </svg>
    {#if prompt.os}
      <p><b>{osTitle(prompt)}</b><span class="line">{osLine(prompt)}</span></p>
    {:else}
      <p><b>{permissionHost(prompt)}</b> wants to {permissionAsk(prompt)}</p>
    {/if}
  </div>
  <div class="buttons">
    {#if prompt.os}
      <button type="button" onclick={() => answer('dismiss')}>Not now</button>
      <button type="button" class="primary" onclick={() => answer('settings')}>Open settings</button>
    {:else}
      <button type="button" onclick={() => answer('block')}>Block</button>
      {#if prompt.once}
        <button type="button" onclick={() => answer('allow')}>Always allow</button>
        <button type="button" class="primary" onclick={() => answer('once')}>Allow this time</button>
      {:else}
        <button type="button" class="primary" onclick={() => answer('allow')}>Allow</button>
      {/if}
    {/if}
  </div>
</Sheet>

<style>
  /* Laid out as the bookmark's sheet is: 20px sides, 48px rows and buttons. */
  .head {
    display: flex;
    gap: 12px;
    padding: 4px 20px 8px;
  }

  .head svg {
    flex: none;
    margin-top: 2px;
    color: var(--icon);
  }

  .head p {
    margin: 0;
    font-size: 15px;
    line-height: 1.4;
    overflow-wrap: anywhere;
  }

  .head b {
    font-weight: 600;
  }

  .line {
    display: block;
    margin-top: 4px;
    color: var(--muted);
    font-size: 13px;
  }

  .buttons {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 8px 20px 4px;
  }

  button {
    min-height: 48px;
    width: 100%;
    padding: 0 18px;
    border: 0;
    border-radius: 10px;
    background: none;
    color: var(--text);
    font: inherit;
    font-size: 15px;
  }

  .primary {
    background: var(--accent);
    color: var(--on-accent);
    font-weight: 600;
  }
</style>
