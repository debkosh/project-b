<script>
  import { focusPage, send, shell } from '@/sdk/svelte.svelte.js';
  import Icon from './Icon.svelte';

  // Over a page that crashed, failed to load or was blocked, which otherwise shows blank:
  // one plain sentence, the error's own name under it, and Try again or Reload; a crashed
  // page also offers Share report, to hand on what the app knows of the crash.
  let { tab } = $props();

  const host = $derived.by(() => {
    try {
      return new URL(tab.url).host.replace(/^www\./, '') || tab.url;
    } catch {
      return tab.url;
    }
  });
  const code = $derived(tab.errorCode || '');
  const said = $derived.by(() => {
    if (tab.blocked) return `${host} is blocked`;
    if (tab.crashed) return 'This page crashed';
    if (tab.error === 'not-found') return `Can't find ${host}`;
    if (tab.error === 'offline') return 'No internet connection';
    return `Can't reach ${host}`;
  });

  function share(e) {
    const r = e.currentTarget.getBoundingClientRect();
    send({ t: 'shareReport', rect: { x: r.x, y: r.y, w: r.width, h: r.height } });
  }

  function again() {
    send({ t: 'reload' });
    focusPage();
  }
</script>

<div class="notice" use:shell role="alert">
  <span class="mark"><Icon name={tab.crashed ? 'reload' : 'warn'} size={32} /></span>
  <p class="said">{said}</p>
  {#if code && !tab.crashed && !tab.blocked}<p class="code">{code}</p>{/if}
  {#if !tab.blocked}
    <button class="text-button primary" onclick={again}>{tab.crashed ? 'Reload' : 'Try again'}</button>
  {/if}
  {#if tab.crashed}
    <button class="text-button quiet" onclick={share}>Share report</button>
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

  p {
    margin: 0;
    overflow-wrap: anywhere;
  }

  .said {
    font-size: 15px;
    font-weight: 600;
  }

  .code {
    color: var(--muted);
    font-size: 12px;
  }

  button {
    margin-top: 12px;
    height: 48px;
    padding: 0 20px;
    border-radius: 20px;
  }
</style>
