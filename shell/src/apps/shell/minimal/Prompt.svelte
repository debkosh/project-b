<script>
  import { activeTab, shell, state } from '@/sdk/svelte.svelte.js';
  import { answerPermission, permissionAsk, permissionHost, promptFor } from '@/sdk/sites.js';
  import { pushEscape } from '@/ui/escape.js';

  // A site asking to use its camera, microphone or notifications, in one line under the
  // toolbar: its question, then Block and Allow, or Block, Always allow and Allow this
  // time where Go offers this time. A dialog named by its question; Escape dismisses it.
  const prompt = $derived(promptFor(state, activeTab(state).id));
  const question = $derived(prompt ? `${permissionHost(prompt)} wants to ${permissionAsk(prompt)}` : '');

  $effect(() => {
    const p = prompt;
    if (p) return pushEscape(() => answerPermission(p, 'dismiss'));
  });
</script>

{#if prompt}
  <div class="prompt" use:shell role="dialog" aria-label={question}>
    <span class="question">{question}</span>
    <button class="words" onclick={() => answerPermission(prompt, 'block')}>Block</button>
    {#if prompt.once}
      <button class="words" onclick={() => answerPermission(prompt, 'allow')}>Always allow</button>
      <button class="words primary" onclick={() => answerPermission(prompt, 'once')}>Allow this time</button>
    {:else}
      <button class="words primary" onclick={() => answerPermission(prompt, 'allow')}>Allow</button>
    {/if}
  </div>
{/if}

<style>
  .prompt {
    display: flex;
    align-items: center;
    gap: 8px;
    flex: none;
    height: 36px;
    padding: 0 4px 0 12px;
    background: var(--bar);
    border-bottom: 1px solid var(--line);
    font-size: 12px;
  }

  .question {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
</style>
