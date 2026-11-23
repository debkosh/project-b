<script>
  import { shell, state as go } from '@/sdk/svelte.svelte.js';
  import { answerPermission, osLine, osTitle, permissionAsk, permissionHost, promptFor } from '@/sdk/sites.js';
  import { pushEscape } from '@/ui/escape.js';

  // A site asking to use its camera, microphone or notifications (docs/daily-driver.md), over
  // the card of the tab that asks, at its top left: the design system's floating surface, as
  // PermissionPrompt.vue is in the Vue shells. A glyph of what is asked, the question, then
  // Block and Allow, or Block, Always allow and Allow this time where Go offers this time.
  // With prompt.os the OS refused what the site is allowed (docs/media-permissions.md): its
  // words, then Not now and Open settings. A dialog named by its question; it takes no focus,
  // and Escape dismisses it while its tab is the active one. tab: the card's tab.
  let { tab } = $props();

  const prompt = $derived(promptFor(go, tab));
  const question = $derived(prompt ? (prompt.os ? osTitle(prompt) : `${permissionHost(prompt)} wants to ${permissionAsk(prompt)}`) : '');
  const glyph = $derived(prompt?.kinds.includes('camera') ? 'camera' : prompt?.kinds.includes('microphone') ? 'microphone' : 'bell');

  $effect(() => {
    const p = prompt;
    if (p && tab === go.active) return pushEscape(() => answerPermission(p, 'dismiss'));
  });
</script>

{#if prompt}
  <div class="prompt" use:shell role="dialog" aria-label={question}>
    <p class="question">
      <svg class="glyph" width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
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
        <span><strong>{osTitle(prompt)}</strong><span class="line">{osLine(prompt)}</span></span>
      {:else}
        <span><strong>{permissionHost(prompt)}</strong> wants to {permissionAsk(prompt)}</span>
      {/if}
    </p>
    <div class="buttons">
      {#if prompt.os}
        <button type="button" class="text-button" onclick={() => answerPermission(prompt, 'dismiss')}>Not now</button>
        <button type="button" class="text-button primary" onclick={() => answerPermission(prompt, 'settings')}>Open settings</button>
      {:else}
        <button type="button" class="text-button" onclick={() => answerPermission(prompt, 'block')}>Block</button>
        {#if prompt.once}
          <button type="button" class="text-button" onclick={() => answerPermission(prompt, 'allow')}>Always allow</button>
          <button type="button" class="text-button primary" onclick={() => answerPermission(prompt, 'once')}>Allow this time</button>
        {:else}
          <button type="button" class="text-button primary" onclick={() => answerPermission(prompt, 'allow')}>Allow</button>
        {/if}
      {/if}
    </div>
  </div>
{/if}

<style>
  .prompt {
    position: absolute;
    top: 12px;
    left: 12px;
    z-index: 2;
    width: 360px;
    max-width: calc(100% - 24px);
    padding: 16px;
    box-sizing: border-box;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--surface);
    box-shadow: var(--shadow-float);
    color: var(--text);
  }

  .question {
    display: flex;
    gap: 10px;
    margin: 0 0 16px;
    font-size: 13px;
    line-height: 1.45;
    overflow-wrap: anywhere;
  }

  .glyph {
    flex: none;
    margin-top: 1px;
    color: var(--icon);
  }

  strong {
    font-weight: 600;
  }

  .line {
    display: block;
    margin-top: 4px;
    color: var(--muted);
  }

  .buttons {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
