<script>
  import { state as go } from '@/sdk/svelte.svelte.js';
  import { linkChoices, openSite, openSiteSettings, permissionWord, setSiteSetting, siteOrigin, siteSetting } from '@/sdk/sites.js';
  import Sheet from './Sheet.svelte';

  // The site panel as a sheet (docs/site-features.md): the active tab's site and its
  // connection; the ad blocker, with what it kept from this page; links to other sites; the
  // sparks the page matches; the permissions the site has set; and Details, to settings.
  // Every change goes to Go, which answers with the site's settings.
  let { tab, onclose } = $props();

  const origin = $derived(siteOrigin(tab));
  const host = $derived(origin.replace(/^https?:\/\//, ''));
  const site = $derived(openSite(go, tab));
  const ads = $derived(siteSetting(site, 'ads'));
  const links = $derived(siteSetting(site, 'links'));
  const permissions = $derived((site?.settings || []).filter((s) => !s.choices && s.value));
  const sparks = $derived(site?.sparks || []);
  const secure = $derived(tab.connection === 'secure');
  // The blocker and a spark change what a page gets as it loads.
  let reload = $state(false);

  function setAds(on) {
    setSiteSetting(origin, 'ads', on ? 'on' : 'off');
    reload = true;
  }
  function setSpark(s, on) {
    setSiteSetting(origin, 'spark:' + s.id, on ? 'on' : 'off');
    reload = true;
  }
  function details() {
    onclose();
    openSiteSettings();
  }
</script>

<Sheet label="Site settings for {host}" {onclose}>
  <div class="head">
    <h2>{host}</h2>
    <p>{secure ? 'Connection is secure' : tab.connectionNote ? `Not secure: ${tab.connectionNote}` : 'Not secure'}</p>
  </div>
  {#if ads}
    <label class="row">
      <span class="text">Ad blocker{#if ads.value === 'on'}<small>{tab.adsBlocked || 0} blocked on this page</small>{/if}</span>
      <input type="checkbox" class="switch" checked={ads.value === 'on'} onchange={(e) => setAds(e.currentTarget.checked)} />
    </label>
  {/if}
  {#if links}
    <div class="row stacked">
      <span class="text">Links to other sites</span>
      <div class="choices" role="radiogroup" aria-label="Links to other sites">
        {#each linkChoices as c (c.value)}
          <button type="button" role="radio" aria-checked={links.value === c.value} onclick={() => setSiteSetting(origin, 'links', c.value)}>{c.label}</button>
        {/each}
      </div>
    </div>
  {/if}
  {#if sparks.length}
    <div role="group" aria-label="Sparks on this site">
      <p class="label">Sparks on this site</p>
      {#each sparks as s (s.id)}
        <label class="row">
          <span class="text">{s.name}</span>
          <input type="checkbox" class="switch" checked={s.on} onchange={(e) => setSpark(s, e.currentTarget.checked)} />
        </label>
      {/each}
    </div>
  {/if}
  {#if permissions.length}
    <div role="group" aria-label="Permissions">
      <p class="label">Permissions</p>
      {#each permissions as p (p.kind)}
        <div class="row"><span class="text">{p.title}</span><span class="value {p.value}">{permissionWord(p.value)}</span></div>
      {/each}
    </div>
  {/if}
  <div class="foot">
    <span>{reload ? 'Changes apply when the page reloads' : ''}</span>
    <button type="button" class="details" onclick={details}>Details</button>
  </div>
</Sheet>

<style>
  .head {
    padding: 4px 20px 12px;
    border-bottom: 1px solid var(--line);
  }
  h2 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    overflow-wrap: anywhere;
  }
  .head p {
    margin: 2px 0 0;
    font-size: 13px;
    color: var(--muted);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 52px;
    padding: 4px 20px;
    font-size: 15px;
  }
  .row.stacked {
    flex-wrap: wrap;
    padding-bottom: 12px;
  }
  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .text small {
    font-size: 13px;
    color: var(--muted);
  }
  .label {
    margin: 0;
    padding: 12px 20px 2px;
    border-top: 1px solid var(--line);
    font-size: 12px;
    font-weight: 600;
    letter-spacing: 0.04em;
    text-transform: uppercase;
    color: var(--muted);
  }
  .choices {
    display: flex;
    width: 100%;
    border: 1px solid var(--line);
    border-radius: 10px;
    overflow: hidden;
  }
  .choices button {
    flex: 1;
    min-height: 44px;
    border: 0;
    background: none;
    color: var(--text);
    font: inherit;
    font-size: 14px;
  }
  .choices button + button {
    border-left: 1px solid var(--line);
  }
  .choices button[aria-checked='true'] {
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 600;
  }
  .value {
    font-size: 14px;
    color: var(--muted);
  }
  .value.allow {
    color: var(--icon);
  }
  .value.block {
    color: var(--danger, #c62828);
  }
  .foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 8px 8px 8px 20px;
    border-top: 1px solid var(--line);
    font-size: 13px;
    color: var(--muted);
  }
  .details {
    min-height: 48px;
    padding: 0 16px;
    border: 0;
    background: none;
    color: var(--accent);
    font: inherit;
    font-weight: 600;
  }
</style>
