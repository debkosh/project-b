<script setup>
// A picture of the browser as the style step's choices make it: tabs on the side, on top,
// or Glide, or a phone with its address bar on top or at the foot. It is drawn, not a
// picture, in the page's own tones, so it takes the colour and the theme as the page does.
defineProps({ layout: String, top: Boolean });
</script>

<template>
  <div v-if="layout === 'phone'" class="phone" aria-hidden="true">
    <div class="screen">
      <div v-if="top" class="address"><i></i></div>
      <div class="page"><u></u><u></u><u class="short"></u></div>
      <div v-if="!top" class="address"><i></i></div>
    </div>
  </div>
  <div v-else class="window" :class="layout" aria-hidden="true">
    <div v-if="layout === 'top'" class="strip">
      <span class="dots"><i></i><i></i><i></i></span>
      <span class="tab on"></span><span class="tab"></span><span class="tab"></span>
    </div>
    <div v-if="layout !== 'glide'" class="bar">
      <span v-if="layout === 'side'" class="dots"><i></i><i></i><i></i></span>
      <span class="field"></span>
    </div>
    <div class="body">
      <div v-if="layout === 'side'" class="rail">
        <span class="tab on"></span><span class="tab"></span><span class="tab"></span><span class="tab"></span>
      </div>
      <div v-if="layout === 'glide'" class="rail glide-rail">
        <span class="dots"><i></i><i></i><i></i></span>
        <span class="field"></span>
        <span class="tab on"></span><span class="tab"></span><span class="tab"></span>
        <span class="spaces"><b class="on"></b><b></b><b></b></span>
      </div>
      <div class="page"><u></u><u></u><u></u><u class="short"></u></div>
    </div>
  </div>
</template>

<style scoped>
.window {
  display: flex;
  flex-direction: column;
  width: 100%;
  aspect-ratio: 16 / 10;
  border: 4px solid var(--frame);
  border-radius: 12px;
  background: var(--bar);
  overflow: hidden;
  transition: border-color var(--motion) var(--ease), background var(--motion) var(--ease);
}

.dots {
  display: flex;
  gap: 4px;
  flex: none;
}

.dots i {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--line);
}

.strip,
.bar {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 28px;
  padding: 0 12px;
  flex: none;
}

.strip {
  gap: 4px;
}

.strip .dots {
  margin-right: 8px;
}

.strip .tab {
  width: 64px;
  height: 18px;
  border-radius: 6px 6px 0 0;
  align-self: flex-end;
}

.field {
  flex: 1;
  height: 16px;
  border-radius: 6px;
  background: var(--surface);
  border: 1px solid var(--line);
}

.body {
  flex: 1;
  display: flex;
  gap: 4px;
  min-height: 0;
  padding: 0 4px 4px;
}

.rail {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 24%;
  padding: 4px;
}

.tab {
  height: 16px;
  border-radius: 6px;
  background: var(--accent-tint);
}

.tab.on {
  background: var(--surface);
  box-shadow: inset 0 0 0 1px var(--line);
}

.glide-rail {
  padding-top: 8px;
}

.glide-rail .field {
  flex: none;
  margin-bottom: 4px;
}

.spaces {
  display: flex;
  gap: 4px;
  margin-top: auto;
  justify-content: center;
}

.spaces b {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--line);
}

.spaces b.on {
  background: var(--accent);
}

.page {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px;
  border-radius: 8px;
  background: var(--surface);
  box-shadow: inset 0 0 0 1px var(--line);
}

.glide .page {
  margin: 8px 4px 4px 0;
}

.page u {
  display: block;
  height: 8px;
  border-radius: 4px;
  background: var(--accent-tint);
}

.page u:first-child {
  width: 40%;
  height: 12px;
  background: var(--accent-soft);
}

.page u.short {
  width: 60%;
}

.phone {
  width: 168px;
  margin: 0 auto;
  padding: 8px;
  border-radius: 24px;
  background: var(--frame);
  transition: background var(--motion) var(--ease);
}

.screen {
  display: flex;
  flex-direction: column;
  aspect-ratio: 9 / 18;
  border-radius: 16px;
  background: var(--bar);
  overflow: hidden;
}

.screen .page {
  border-radius: 0;
  box-shadow: none;
}

.address {
  flex: none;
  padding: 8px;
}

.address i {
  display: block;
  height: 20px;
  border-radius: 10px;
  background: var(--surface);
  border: 1px solid var(--line);
}
</style>
