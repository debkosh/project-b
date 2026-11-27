<script>
  import { focusPage, send, shell } from '@/sdk/svelte.svelte.js';
  import { runNotice } from '@/sdk/notice.js';
  import Icon from './Icon.svelte';

  // Over a page that failed, crashed, was blocked or has a bad certificate, which otherwise
  // shows blank: Go's notice (docs/daily-driver.md), its sentence, its second line and its
  // actions as buttons, the primary one first. A crashed page also offers Share report, to
  // hand on what the app knows of the crash, and Save report, to keep it in Downloads.
  let { tab } = $props();
  const notice = $derived(tab.notice);

  function share(e) {
    const r = e.currentTarget.getBoundingClientRect();
    send({ t: 'shareReport', rect: { x: r.x, y: r.y, w: r.width, h: r.height } });
  }

  function run(id) {
    runNotice(tab, id);
    if (id !== 'back') focusPage();
  }
</script>

<div class="notice" use:shell role="alert" data-kind={notice.kind}>
  <span class="mark"><Icon name={notice.kind === 'crashed' ? 'reload' : 'warn'} size={32} /></span>
  <p class="said">{notice.title}</p>
  {#if notice.detail}<p class="code">{notice.detail}</p>{/if}
  {#if notice.actions.length || notice.kind === 'crashed'}
    <div class="actions">
      {#each notice.actions as a (a.id)}
        <button class="text-button" class:primary={a.primary} class:quiet={!a.primary} onclick={() => run(a.id)}>{a.title}</button>
      {/each}
      {#if notice.kind === 'crashed'}
        <button class="text-button quiet" onclick={share}>Share report</button>
        <button class="text-button quiet" onclick={() => send({ t: 'saveReport' })}>Save report</button>
      {/if}
    </div>
  {/if}
</div>

<style>
  .notice {
    position: absolute;
    inset: 0;
    z-index: 2;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 24px;
    background: var(--bar);
    color: var(--text);
    text-align: center;
  }

  .mark {
    color: var(--muted);
  }

  [data-kind='certificate'] .mark,
  [data-kind='unsafe'] .mark {
    color: var(--danger);
  }

  p {
    margin: 0;
    overflow-wrap: anywhere;
  }

  .said {
    font-size: calc(15 * var(--capped-rem) / 16);
    font-weight: 600;
  }

  .code {
    max-width: 320px;
    color: var(--muted);
    font-size: calc(12 * var(--capped-rem) / 16);
    line-height: 1.45;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
    margin-top: 12px;
  }

  button {
    height: 48px;
    padding: 0 20px;
    border-radius: 20px;
  }
</style>
