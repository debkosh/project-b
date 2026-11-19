<script>
  import { send, shell } from '@/sdk/svelte.svelte.js';
  import { paint, swatches } from '@/ui/hues.js';
  import * as g from './glide.svelte.js';

  // A name and a colour, for a Space or a folder: {title, name, color, create, save}. The
  // colour is one of gulal's seven hues, or any from the system's picker behind the +.
  let { dialog } = $props();
  // A dialog is made for one thing and goes when it is answered.
  // svelte-ignore state_referenced_locally
  let name = $state(dialog.name || '');
  // svelte-ignore state_referenced_locally
  let color = $state(dialog.color);
  let field;

  $effect(() => {
    send({ t: 'focusShell' });
    field.focus();
    field.select();
  });

  function done(ok) {
    if (ok && (name.trim() || !dialog.create)) dialog.save(name.trim(), color);
    g.ui.dialog = null;
  }
</script>

<div class="scrim" use:shell role="dialog" aria-modal="true" aria-label={dialog.title} tabindex="-1" onpointerdown={(e) => e.target === e.currentTarget && done(false)} onkeydown={(e) => e.key === 'Escape' && done(false)}>
  <form
    class="dialog"
    aria-label={dialog.title}
    onsubmit={(e) => {
      e.preventDefault();
      done(true);
    }}
  >
    <h2>{dialog.title}</h2>
    <input bind:this={field} bind:value={name} aria-label="Name" placeholder="Name" autocomplete="off" spellcheck="false" />
    <div class="swatches" role="radiogroup" aria-label="Colour">
      {#each swatches() as c (c.color)}
        <button type="button" class="swatch" role="radio" aria-checked={c.color === color} aria-label={c.name} style:background={c.paint} onclick={() => (color = c.color)}></button>
      {/each}
      <label class="swatch custom" class:on={color?.startsWith('#')} style:background={color?.startsWith('#') ? color : null} title="Custom colour">
        <input type="color" value={color?.startsWith('#') ? color : '#808080'} aria-label="Custom colour" oninput={(e) => (color = e.currentTarget.value)} />
      </label>
    </div>
    <div class="answers">
      <button type="button" class="text-button" onclick={() => done(false)}>Cancel</button>
      <button type="submit" class="text-button primary" disabled={dialog.create && !name.trim()}>{dialog.create ? 'Create' : 'Save'}</button>
    </div>
  </form>
</div>

<style>
  .scrim {
    position: fixed;
    inset: 0;
    display: grid;
    place-items: start center;
    padding-top: 120px;
    background: var(--g-scrim);
  }

  .dialog {
    width: 320px;
    padding: 16px;
    border: 1px solid var(--line);
    border-radius: 8px;
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--g-float-shadow);
  }

  h2 {
    margin: 0 0 12px;
    font-size: 14px;
    font-weight: 600;
  }

  input {
    display: block;
    width: 100%;
    height: 32px;
    box-sizing: border-box;
    padding: 0 10px;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--surface);
    color: var(--text);
    font: inherit;
    outline: none;
  }

  input:focus {
    box-shadow: var(--focus-ring);
  }

  .swatches {
    display: flex;
    gap: 8px;
    margin: 12px 0 16px;
  }

  .swatch {
    width: 24px;
    height: 24px;
    padding: 0;
    border: 0;
    border-radius: 50%;
  }

  .swatch[aria-checked='true'],
  .swatch.custom.on {
    box-shadow: 0 0 0 2px var(--surface), 0 0 0 4px var(--text);
  }

  /* The custom swatch: a + on the surface until a colour is picked with it. */
  .swatch.custom {
    position: relative;
    display: grid;
    place-items: center;
    box-shadow: inset 0 0 0 1px var(--line);
    background: var(--surface);
    cursor: pointer;
  }

  .swatch.custom::before,
  .swatch.custom::after {
    content: '';
    position: absolute;
    width: 10px;
    height: 1.5px;
    border-radius: 1px;
    background: var(--icon);
  }

  .swatch.custom::after {
    transform: rotate(90deg);
  }

  .swatch.custom.on::before,
  .swatch.custom.on::after {
    background: var(--surface);
  }

  .swatch.custom input {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    margin: 0;
    padding: 0;
    border: 0;
    opacity: 0;
    cursor: pointer;
  }

  .answers {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
