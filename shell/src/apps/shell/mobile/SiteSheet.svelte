<script>
  import { state as go } from '@/sdk/svelte.svelte.js';
  import { linkChoices, openSite, openSiteSettings, permissionWord, reloadPage, setSiteSetting, siteOrigin, siteSetting, sparkNames } from '@/sdk/sites.js';
  import Sheet from './Sheet.svelte';

  // The site panel as a sheet (docs/site-features.md): the active tab's site and its
  // connection; the ad blocker, with what it kept from this page; whether sparks run on the
  // site; links to other sites; the permissions the site has set; and Details, to settings.
  // Every change goes to Go, which answers with the site's settings.
  let { tab, onclose } = $props();

  const origin = $derived(siteOrigin(tab));
  const host = $derived(origin.replace(/^https?:\/\//, ''));
  const site = $derived(openSite(go, tab));
  const ads = $derived(siteSetting(site, 'ads'));
  const links = $derived(siteSetting(site, 'links'));
  const permissions = $derived((site?.settings || []).filter((s) => !s.choices && s.value));
  const sparksOn = $derived(siteSetting(site, 'sparks'));
  // The sparks that run here while the site's sparks are on, by name.
  const sparks = $derived(site?.sparks || []);
  const secure = $derived(tab.connection === 'secure');
  // The blocker and a spark change what a page gets as it loads.
  let reload = $state(false);

  function setAds(on) {
    setSiteSetting(origin, 'ads', on ? 'on' : 'off');
    reload = true;
  }
  function setSparks(on) {
    setSiteSetting(origin, 'sparks', on ? 'on' : 'off');
    reload = true;
  }
  function reloadNow() {
    reloadPage();
    reload = false;
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
      <span class="text"><span>Ad blocker</span>{#if ads.value === 'on'}<small id="site-ads-note">{tab.adsBlocked || 0} blocked on this page</small>{/if}</span>
      <input
        type="checkbox"
        role="switch"
        class="switch"
        aria-label="Ad blocker"
        aria-describedby={ads.value === 'on' ? 'site-ads-note' : undefined}
        checked={ads.value === 'on'}
        aria-checked={String(ads.value === 'on')}
        onchange={(e) => setAds(e.currentTarget.checked)}
      />
    </label>
  {/if}
  {#if sparksOn && sparks.length}
    <label class="row">
      <span class="text"><span>Sparks</span><small id="site-sparks-note">{sparkNames(sparks)}</small></span>
      <input
        type="checkbox"
        role="switch"
        class="switch"
        aria-label="Sparks"
        aria-describedby="site-sparks-note"
        checked={sparksOn.value === 'on'}
        aria-checked={String(sparksOn.value === 'on')}
        onchange={(e) => setSparks(e.currentTarget.checked)}
      />
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
    {#if reload}<button type="button" class="details" onclick={reloadNow}>Reload</button>{/if}
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
  /* The choice that is on: filled, as the switches beside it are, readable in both themes. */
  .choices button[aria-checked='true'] {
    background: var(--accent);
    color: var(--on-accent);
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
    color: var(--danger);
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

  .foot > span {
    flex: 1;
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
