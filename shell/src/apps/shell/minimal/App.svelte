<script>
  import { untrack } from 'svelte';
  import { activeTab, connect, focusPage, keyLabel, page, send, setFrame, shell, state as go } from '@/sdk/svelte.svelte.js';
  import Address from './Address.svelte';
  import CrashBar from './CrashBar.svelte';
  import Notice from './Notice.svelte';
  import PageDialog from './PageDialog.svelte';
  import PageMenu from './PageMenu.svelte';
  import PageTitle from './PageTitle.svelte';
  import Prompt from './Prompt.svelte';
  import SideList from './SideList.svelte';
  import TabBar from './TabBar.svelte';
  import Toolbar from './Toolbar.svelte';

  // The minimal shell: a tab bar, a toolbar under it, and the page, square and flat. With
  // the tabs down the side the tab bar goes, the toolbar is the top row, and the tabs are a
  // list on the left. The bars and the page reach the window's edges: it asks for no band.
  setFrame(0);

  const horizontal = $derived(go.tabLayout === 'horizontal');
  let address = $state();

  // A tab that becomes active with nothing in it wants the address; one with a page wants
  // the page. What was typed in the field was for the tab before. The shell has to hold
  // the keyboard before its field can. A window left with no tabs wants the address too. It
  // runs once the view has drawn the change, and the view is made once Go's data is in, so
  // the state it is made in counts too. One of our tabs has no field: its title is there.
  function activated() {
    address?.blur();
    if (activeTab(go).url === '') {
      send({ t: 'focusShell' });
      address.focus();
    } else {
      focusPage();
    }
  }

  $effect(() => {
    go.active;
    if (go.loaded) untrack(activated);
  });

  connect({
    focusAddressBar: () => address?.focus(),
    suggestions: (m) => address?.suggestions(m),
  });
</script>

<!-- Until Go's snapshot is in, the shell knows nothing of the window: its splash is the
     window's own colour, which the engine paints under the shell, and the app's name. When
     the data comes the UI is made, and the two cross-fade. -->
<div class="splash" aria-busy={!go.loaded}>Project B</div>
{#if go.loaded}
  <div class="window">
    {#if horizontal}
      <TabBar />
    {/if}
    <Toolbar vertical={!horizontal}>
      {#if activeTab(go).page}
        <PageTitle />
      {:else}
        <Address bind:this={address} url={activeTab(go).url} tab={activeTab(go)} />
      {/if}
    </Toolbar>
    <CrashBar />
    <Prompt />
    <div class="body">
      {#if !horizontal}
        <SideList />
      {/if}
      <!-- With no tabs the shell draws the page's area itself, as its own. -->
      <main use:page={go.active}>
        {#if !go.tabs.length}
          <div class="empty" use:shell>
            <p>No tabs</p>
            <button class="words" type="button" onclick={() => send({ t: 'newTab' })}>
              New Tab{#if keyLabel(go, 'tab.new')}<span class="key" aria-hidden="true">{keyLabel(go, 'tab.new')}</span>{/if}
            </button>
          </div>
        {/if}
        {#if activeTab(go).notice}
          <Notice tab={activeTab(go)} />
        {/if}
        <PageDialog />
      </main>
    </div>
    <PageMenu />
  </div>
{/if}

<style>
  .window {
    display: flex;
    flex-direction: column;
    height: 100%;
    font: 13px/1 var(--sans);
    color: var(--text);
    transition: opacity var(--motion) var(--ease);
  }

  /* The UI fades in as it is made, as the splash fades out. */
  @starting-style {
    .window {
      opacity: 0;
    }
  }

  /* The splash is only a picture, so a press goes through it to the UI while it fades; then
     it goes. In the shell's own type. */
  .splash {
    position: fixed;
    inset: 0;
    display: grid;
    place-items: center;
    font: 500 15px/1 var(--sans);
    letter-spacing: 0.02em;
    color: rgb(255 255 255 / 0.85);
    user-select: none;
    pointer-events: none;
    transition: opacity var(--motion) var(--ease), display var(--motion) allow-discrete;
  }

  .splash[aria-busy='false'] {
    display: none;
    opacity: 0;
  }

  .body {
    display: flex;
    flex: 1;
    min-height: 0;
  }

  main {
    position: relative;
    flex: 1;
    min-width: 0;
  }

  .empty {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    background: var(--surface);
  }

  .empty p {
    margin: 0;
    color: var(--muted);
  }

  .key {
    margin-left: 8px;
    color: var(--muted);
    font-weight: 400;
  }
</style>
